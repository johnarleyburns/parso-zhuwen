// HEIC encode stage for the §8A Commons image pipeline (handoff §1 external-build-time-stage
// pattern, like the TTS/aligner stage). It downloads each chosen Commons file, resizes to the
// target size, HEIC-encodes it, and populates pack.Image.Data so packs ship with real covers.
//
// Two modes:
//   - EncodeModeStub — deterministic, hermetic, no network / no shell-out. Produces a
//     deterministic marker blob (or representative-weight blob for budget exercises) so CI can
//     exercise the full size-budget assertion with realistic HEIC weights (I2: no CI network).
//   - EncodeModeReal — shells out to a local Python script that downloads the original Commons
//     file (network, build-time only, never in CI), resizes, and HEIC-encodes it at the target
//     pixel size. The script contract is a JSON-in/JSON-out protocol on temp files.
//
// The app never runs this package (I3): images are pregenerated here and shipped in the .zpack.
package images

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/parso/zhuwen-factory/internal/pack"
)

// EncodeMode selects the encode backend.
type EncodeMode int

const (
	EncodeModeStub EncodeMode = iota
	EncodeModeReal
)

func (m EncodeMode) String() string {
	if m == EncodeModeReal {
		return "real"
	}
	return "stub"
}

// EncodeConfig parameterizes the HEIC encode stage.
type EncodeConfig struct {
	Mode     EncodeMode
	TargetPX int // longest-edge target, e.g. 480

	// RepresentativeStub, when true, makes the stub emit bytes of realistic HEIC weight
	// (~120 KB at 480px) instead of a compact marker. Kept false for vendored fixtures;
	// the size-budget test sets it (and builds its own weights).
	RepresentativeStub bool

	// ModeReal shell-out configuration (ignored under EncodeModeStub).
	PythonBin string // interpreter for the image-processing venv
	Script    string // path to the download+resize+encode script (emits EncodeJSON on stdout)
	WorkDir   string // working directory for the script
}

// DefaultEncodeConfig is the hermetic stub path.
func DefaultEncodeConfig() EncodeConfig {
	return EncodeConfig{
		Mode:     EncodeModeStub,
		TargetPX: 480,
	}
}

// EncodeJSON is the contract the external ModeReal script emits on stdout: the script
// writes the HEIC file to the out_dir it received and reports the path + dimensions.
type EncodeJSON struct {
	HeicPath string `json:"heic_path"`
	W        int    `json:"w"`
	H        int    `json:"h"`
}

// encodeInput is written to a temp file as JSON for the external script.
type encodeInput struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	SourceURL string `json:"source_url"`
	TargetPX  int    `json:"target_px"`
	OutDir    string `json:"out_dir"`
}

// RepresentativeHeicByteLen is the representative on-disk size (bytes) of a 480px
// HEIC cover image. This model is used for size-budget exercises so the NFR-3/NFR-4
// assertion reflects real HEIC weights, not the compact CI stub. Based on
// CP-08a/CP-09c estimates (~120 KB per cover).
const RepresentativeHeicByteLen = 120 * 1000

// EncodeImage encodes a single Commons image: in stub mode it produces a deterministic
// marker; in real mode it shells out to a Python script that downloads, resizes, and
// HEIC-encodes the file.
func EncodeImage(id, title, sourceURL string, cfg EncodeConfig) ([]byte, int, int, error) {
	if cfg.TargetPX == 0 {
		cfg.TargetPX = 480
	}
	switch cfg.Mode {
	case EncodeModeReal:
		return encodeReal(id, title, sourceURL, cfg)
	default:
		data := encodeStub(id, cfg.RepresentativeStub)
		return data, cfg.TargetPX, cfg.TargetPX, nil
	}
}

// EncodePackImages encodes every image in a slice, populating Data in place. Images that
// already have Data set (non-nil) are skipped.
func EncodePackImages(images []pack.Image, cfg EncodeConfig) ([]pack.Image, error) {
	for i := range images {
		if images[i].Data != nil {
			continue
		}
		data, w, h, err := EncodeImage(images[i].ID, images[i].SourceURL, images[i].SourceURL, cfg)
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", images[i].ID, err)
		}
		images[i].Data = data
		images[i].W = w
		images[i].H = h
	}
	return images, nil
}

// encodeStub generates deterministic byte content for an image. Content is a SHA-256
// keystream seeded by image ID so identical IDs yield identical bytes (hermetic determinism).
// By default the stub is compact (a small marker blob) so vendored fixtures stay tiny; set
// representative=true to emit bytes of realistic HEIC weight for size-budget exercises.
func encodeStub(id string, representative bool) []byte {
	n := len("HeicZhuwenStub\x00") + 32
	if representative {
		n = RepresentativeHeicByteLen
	}
	prefix := []byte("HeicZhuwenStub\x00")
	if n < len(prefix) {
		n = len(prefix)
	}
	out := make([]byte, 0, n)
	out = append(out, prefix...)
	var counter uint64
	seed := id
	for len(out) < n {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s#%d", seed, counter)))
		out = append(out, h[:]...)
		counter++
	}
	return out[:n]
}

// encodeReal shells out to the local image-processing script (build-time only, I2). It
// writes an encode input JSON to a temp file, runs the script, and reads back the HEIC
// bytes from the reported path.
func encodeReal(id, title, sourceURL string, cfg EncodeConfig) ([]byte, int, int, error) {
	if cfg.PythonBin == "" || cfg.Script == "" {
		return nil, 0, 0, fmt.Errorf("images encode: ModeReal requires PythonBin and Script")
	}
	tmp, err := os.MkdirTemp("", "zhuwen-encode-*")
	if err != nil {
		return nil, 0, 0, err
	}
	defer os.RemoveAll(tmp)

	inPath := filepath.Join(tmp, "in.json")
	in := encodeInput{
		ID:        id,
		Title:     title,
		SourceURL: sourceURL,
		TargetPX:  cfg.TargetPX,
		OutDir:    tmp,
	}
	inBytes, _ := json.MarshalIndent(in, "", "  ")
	if err := os.WriteFile(inPath, inBytes, 0o600); err != nil {
		return nil, 0, 0, err
	}

	cmd := exec.Command(cfg.PythonBin, cfg.Script, "--in", inPath)
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}
	stdout, err := cmd.Output()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("images encode: script failed: %w (output: %s)", err, string(stdout))
	}
	var ej EncodeJSON
	if err := json.Unmarshal(stdout, &ej); err != nil {
		return nil, 0, 0, fmt.Errorf("images encode: parse script output: %w", err)
	}
	data, err := os.ReadFile(ej.HeicPath)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("images encode: read HEIC file: %w", err)
	}
	return data, ej.W, ej.H, nil
}
