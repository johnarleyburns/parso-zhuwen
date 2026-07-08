// Package tts is the factory's build-time text-to-speech render stage (handoff §1, §4.7).
// It turns a segmented story into (a) narration audio bytes (24 kbps mono Opus, the NFR-4
// budget basis) and (b) word-level alignment rows under the frozen pack.AlignToken contract
// (one strictly-increasing, non-overlapping row per token, contiguous within a sentence).
//
// Two modes, selected by Config.Mode (handoff §1 external-stage pattern):
//
//   - ModeStub — deterministic, hermetic, no network / no venv. Timings come from the
//     internal/align character-rate model; audio bytes are a deterministic placeholder of a
//     *representative* 24 kbps size so the NFR-3/NFR-4 size budget (internal/pack) is exercised
//     with realistic weights even in CI. This is the CI/dev path (I2: no app or CI network).
//   - ModeReal — shells out to a local CosyVoice 3.0 render + forced aligner (Apple Silicon
//     MPS, build-time only, $0; never linked into the app — I3/I2). The real word timings
//     *replace* the character-rate model output under the identical AlignToken contract; the
//     Opus bytes replace the CP-06 fixture stub. Voice-model license is recorded before this
//     path ships (blockers.md B-5). Never exercised in CI.
//
// The app never runs this package and never computes timing (I3): audio + alignment are
// pregenerated here and shipped in the .zpack.
package tts

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/parso/zhuwen-factory/internal/align"
	"github.com/parso/zhuwen-factory/internal/pack"
	"github.com/parso/zhuwen-factory/internal/segment"
)

// Mode selects the render backend.
type Mode int

const (
	// ModeStub is the deterministic, hermetic CI/dev path.
	ModeStub Mode = iota
	// ModeReal shells out to the local CosyVoice render + aligner.
	ModeReal
)

func (m Mode) String() string {
	if m == ModeReal {
		return "real"
	}
	return "stub"
}

// Config parameterizes a render.
type Config struct {
	Mode         Mode
	Voice        string       // voice-model id (provenance; e.g. "cosyvoice3-female-zh-01")
	Model        string       // render model id (provenance; e.g. "cosyvoice-3.0")
	SampleRateHz int          // e.g. 24000
	BitrateBps   int          // Opus target bitrate; 24000 = 24 kbps (the NFR-4 basis)
	AlignCfg     align.Config // stub timing model (also the CI fallback under ModeReal wiring)

	// RepresentativeStub, when true, makes the stub emit audio bytes of realistic 24 kbps
	// weight (StubByteLen) instead of a compact marker. Kept false for vendored fixtures so
	// ios/Fixtures stays tiny; the size-budget test sets it (and builds its own weights).
	RepresentativeStub bool

	// ModeReal shell-out configuration (ignored under ModeStub).
	PythonBin string // interpreter for the CosyVoice venv, e.g. "./venv/bin/python"
	Script    string // path to the render+align script (emits RenderJSON on stdout)
	WorkDir   string // working directory for the render process
}

// DefaultConfig is the hermetic stub path at the 24 kbps mono NFR-4 basis.
func DefaultConfig() Config {
	return Config{
		Mode:         ModeStub,
		Voice:        "cosyvoice3-female-zh-01",
		Model:        "cosyvoice-3.0",
		SampleRateHz: 24000,
		BitrateBps:   24000,
		AlignCfg:     align.DefaultConfig(),
	}
}

// Provenance records how a render was produced (recorded alongside the pack, I4).
type Provenance struct {
	Tool         string `json:"tool"`  // "cosyvoice" | "cosyvoice-stub"
	Model        string `json:"model"` // model id
	Voice        string `json:"voice"`
	SampleRateHz int    `json:"sample_rate_hz"`
	BitrateBps   int    `json:"bitrate_bps"`
	Mode         string `json:"mode"` // "stub" | "real"
}

// Result is a rendered story.
type Result struct {
	OpusBytes  []byte
	Timings    []pack.AlignToken
	DurationMs int
	Provenance Provenance
}

// RenderJSON is the contract the external ModeReal script emits on stdout: base64 is avoided
// by writing audio to a side file; the script reports its path plus the word timings.
type RenderJSON struct {
	AudioPath  string            `json:"audio_path"`
	DurationMs int               `json:"duration_ms"`
	Timings    []pack.AlignToken `json:"timings"`
}

// Render produces narration audio + word-level alignment for one story's tokens. storyID keys
// the deterministic stub (identical input → identical bytes) and the real render's temp files.
func Render(tokens []segment.Token, storyID string, cfg Config) (Result, error) {
	if cfg.SampleRateHz == 0 {
		cfg.SampleRateHz = 24000
	}
	if cfg.BitrateBps == 0 {
		cfg.BitrateBps = 24000
	}
	switch cfg.Mode {
	case ModeReal:
		return renderReal(tokens, storyID, cfg)
	default:
		return renderStub(tokens, storyID, cfg), nil
	}
}

// renderStub is the deterministic hermetic path.
func renderStub(tokens []segment.Token, storyID string, cfg Config) Result {
	rows, totalMs := align.Align(tokens, cfg.AlignCfg)
	return Result{
		OpusBytes:  stubOpus(storyID, totalMs, cfg.BitrateBps, cfg.RepresentativeStub),
		Timings:    rows,
		DurationMs: totalMs,
		Provenance: Provenance{
			Tool:         "cosyvoice-stub",
			Model:        cfg.Model,
			Voice:        cfg.Voice,
			SampleRateHz: cfg.SampleRateHz,
			BitrateBps:   cfg.BitrateBps,
			Mode:         "stub",
		},
	}
}

// StubByteLen is the representative on-disk size (bytes) of totalMs of Opus at bitrateBps:
// bitrate/8 bytes per second. This is the weight *model* used by pack.MeasureBudget's audio
// projection and by the size-budget test's realistic synthetic packs (so the NFR-3/NFR-4
// assertion reflects real 24 kbps weights, not the compact CI stub).
func StubByteLen(totalMs, bitrateBps int) int {
	if totalMs <= 0 || bitrateBps <= 0 {
		return 0
	}
	return totalMs * bitrateBps / 8 / 1000
}

// stubOpus generates deterministic byte content for a story. Content is a SHA-256 keystream
// seeded by storyID so identical stories yield identical bytes (hermetic determinism). By
// default the stub is *compact* (a small marker blob) so vendored fixtures stay tiny; set
// representative=true to emit bytes of realistic 24 kbps weight for size-budget exercises.
func stubOpus(storyID string, totalMs, bitrateBps int, representative bool) []byte {
	n := len("OpusZhuwenStub\x00") + 32
	if representative {
		n = StubByteLen(totalMs, bitrateBps)
	}
	prefix := []byte("OpusZhuwenStub\x00")
	if n < len(prefix) {
		n = len(prefix)
	}
	out := make([]byte, 0, n)
	out = append(out, prefix...)
	var counter uint64
	seed := storyID
	for len(out) < n {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s#%d", seed, counter)))
		out = append(out, h[:]...)
		counter++
	}
	return out[:n]
}

// renderReal shells out to the local CosyVoice render + aligner (build-time only, I2). It
// writes the story text to a temp file, runs the script, and reads back audio + timings. The
// timings replace the character-rate model output under the same AlignToken contract (I3).
func renderReal(tokens []segment.Token, storyID string, cfg Config) (Result, error) {
	if cfg.PythonBin == "" || cfg.Script == "" {
		return Result{}, fmt.Errorf("tts: ModeReal requires PythonBin and Script (see blockers.md B-5)")
	}
	tmp, err := os.MkdirTemp("", "zhuwen-tts-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(tmp)

	inPath := filepath.Join(tmp, "in.json")
	type renderInput struct {
		StoryID      string   `json:"story_id"`
		Voice        string   `json:"voice"`
		SampleRateHz int      `json:"sample_rate_hz"`
		BitrateBps   int      `json:"bitrate_bps"`
		Tokens       []string `json:"tokens"`
		Sentences    []int    `json:"sentence_idx"`
	}
	ri := renderInput{StoryID: storyID, Voice: cfg.Voice, SampleRateHz: cfg.SampleRateHz, BitrateBps: cfg.BitrateBps}
	for _, t := range tokens {
		ri.Tokens = append(ri.Tokens, t.Text)
		ri.Sentences = append(ri.Sentences, t.SentenceIdx)
	}
	inBytes, _ := json.MarshalIndent(ri, "", "  ")
	if err := os.WriteFile(inPath, inBytes, 0o600); err != nil {
		return Result{}, err
	}

	cmd := exec.Command(cfg.PythonBin, cfg.Script, "--in", inPath, "--out-dir", tmp)
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}
	stdout, err := cmd.Output()
	if err != nil {
		return Result{}, fmt.Errorf("tts: CosyVoice render failed: %w", err)
	}
	var rj RenderJSON
	if err := json.Unmarshal(stdout, &rj); err != nil {
		return Result{}, fmt.Errorf("tts: parse render output: %w", err)
	}
	audio, err := os.ReadFile(rj.AudioPath)
	if err != nil {
		return Result{}, fmt.Errorf("tts: read rendered audio: %w", err)
	}
	if err := ValidateTimings(rj.Timings, len(tokens)); err != nil {
		return Result{}, fmt.Errorf("tts: real alignment violates AlignToken contract: %w", err)
	}
	return Result{
		OpusBytes:  audio,
		Timings:    rj.Timings,
		DurationMs: rj.DurationMs,
		Provenance: Provenance{
			Tool:         "cosyvoice",
			Model:        cfg.Model,
			Voice:        cfg.Voice,
			SampleRateHz: cfg.SampleRateHz,
			BitrateBps:   cfg.BitrateBps,
			Mode:         "real",
		},
	}, nil
}

// ValidateTimings enforces the pack.AlignToken contract on a set of rows (shared by the real
// path and tests): one row per token, token_idx == position, strictly increasing and
// non-overlapping in time. This is the same invariant internal/align guarantees for the stub;
// the real aligner's output must satisfy it before it can ship (I3).
func ValidateTimings(rows []pack.AlignToken, wantTokens int) error {
	if wantTokens >= 0 && len(rows) != wantTokens {
		return fmt.Errorf("expected %d rows (one per token), got %d", wantTokens, len(rows))
	}
	for i, r := range rows {
		if r.TokenIdx != i {
			return fmt.Errorf("row %d has token_idx %d (want %d)", i, r.TokenIdx, i)
		}
		if r.T1ms <= r.T0ms {
			return fmt.Errorf("row %d non-positive duration [%d,%d)", i, r.T0ms, r.T1ms)
		}
		if i > 0 && r.T0ms < rows[i-1].T1ms {
			return fmt.Errorf("row %d overlaps previous: t0=%d < prev t1=%d", i, r.T0ms, rows[i-1].T1ms)
		}
	}
	return nil
}
