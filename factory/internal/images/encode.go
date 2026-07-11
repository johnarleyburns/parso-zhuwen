// HEIC encode stage for the §8A Commons image pipeline (handoff §1 external-build-time-stage
// pattern, like the TTS/aligner stage). It downloads each chosen Commons file, resizes to the
// target size, HEIC-encodes it, and populates pack.Image.Data so packs ship with real covers.
//
// Two modes:
//   - EncodeModeStub — deterministic, hermetic, no network / no shell-out. Produces a
//     tiny valid PNG with a color derived from the image ID so each image is distinct and
//     decodable on-device. A representative-weight mode pads to realistic HEIC-weights for
//     size-budget exercises (I2: no CI network).
//   - EncodeModeReal — shells out to a local Python script that downloads the original Commons
//     file (network, build-time only, never in CI), resizes, and HEIC-encodes it at the target
//     pixel size. The script contract is a JSON-in/JSON-out protocol on temp files.
//
// The app never runs this package (I3): images are pregenerated here and shipped in the .zpack.
package images

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
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

// encodeStub generates a tiny valid PNG image whose dominant color is derived
// deterministically from the image ID, so every stub image is distinct and decodable
// on-device (UIImage/NSImage can decode PNG regardless of file extension). In
// representative mode the PNG is padded with a trailing PNG comment chunk to reach
// realistic HEIC-weights for size-budget exercises while remaining a valid PNG.
func encodeStub(id string, representative bool) []byte {
	// Deterministic hue from first bytes of SHA-256 of the image ID.
	h := sha256.Sum256([]byte(id))
	r := uint8(h[0])
	g := uint8(h[1])
	b := uint8(h[2])

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: r, G: g, B: b, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		// Unreachable for a 1x1 image in memory.
		panic("png encode: " + err.Error())
	}
	data := buf.Bytes()

	if representative {
		// Pad to realistic HEIC weight by appending PNG tEXt comment chunks
		// filled with deterministic filler. The file stays a valid PNG but
		// reaches the exact on-disk size of a real 480px HEIC cover.
		// Each tEXt chunk adds 12 bytes overhead (length + type + CRC).
		// We compute payload size to achieve exactly the target total.
		payloadNeeded := RepresentativeHeicByteLen - len(data)
		if payloadNeeded > 0 {
			const chunkOverhead = 12 // 4 length + 4 type + 4 CRC
			chunkPayloadMax := 8192  // keep chunks reasonably sized
			var chunks [][]byte
			remaining := payloadNeeded
			for remaining > 0 {
				// Overshoot slightly so after overhead the total lands exactly
				// at target. We adjust the last chunk to make it exact.
				n := remaining
				if n > chunkPayloadMax {
					// Account for this chunk's overhead in remaining budget.
					n = chunkPayloadMax
				}
				key := fmt.Sprintf("pad%04x", remaining)
				chunk := buildPNGTextChunkChunk(key, deterministicFill(key, n))
				chunks = append(chunks, chunk)
				remaining -= n + chunkOverhead
			}
			// If we overshot (rare: last chunk payload pushed past), trim the last
			// chunk's payload to land exactly at target.
			total := len(data)
			for _, c := range chunks {
				total += len(c)
			}
			if total != RepresentativeHeicByteLen {
				chunks[len(chunks)-1] = trimChunk(chunks[len(chunks)-1], total-RepresentativeHeicByteLen)
			}
			for _, c := range chunks {
				data = append(data, c...)
			}
		}
	}
	return data
}

// buildPNGTextChunkChunk builds a complete PNG tEXt chunk (length + type + data + CRC).
func buildPNGTextChunkChunk(keyword, text string) []byte {
	data := append([]byte(keyword+"\x00"), []byte(text)...)
	return buildPNGChunk('t', 'E', 'X', 't', data)
}

// deterministicFill returns deterministic bytes for padding.
func deterministicFill(seed string, n int) string {
	b := make([]byte, n)
	var counter uint64
	for i := 0; i < n; i++ {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s#%d", seed, counter)))
		copy(b[i:], h[:])
		i += len(h) - 1
		if i >= n {
			break
		}
		counter++
	}
	return string(b[:n])
}

// buildPNGChunk builds a single PNG chunk (length + type + data + CRC).
func buildPNGChunk(t0, t1, t2, t3 byte, data []byte) []byte {
	chunk := make([]byte, 8+len(data))
	chunk[0] = byte(len(data) >> 24)
	chunk[1] = byte(len(data) >> 16)
	chunk[2] = byte(len(data) >> 8)
	chunk[3] = byte(len(data))
	chunk[4] = t0
	chunk[5] = t1
	chunk[6] = t2
	chunk[7] = t3
	copy(chunk[8:], data)
	// CRC-32 of type + data.
	crc := pngCRC(append([]byte{t0, t1, t2, t3}, data...))
	chunk = append(chunk, byte(crc>>24), byte(crc>>16), byte(crc>>8), byte(crc))
	return chunk
}

// pngCRC computes the CRC-32 used by PNG chunks.
func pngCRC(data []byte) uint32 {
	// PNG uses ISO 3309 CRC, standard CRC-32.
	var c uint32 = 0xffffffff
	for _, b := range data {
		c ^= uint32(b)
		for i := 0; i < 8; i++ {
			if c&1 != 0 {
				c = (c >> 1) ^ 0xedb88320
			} else {
				c >>= 1
			}
		}
	}
	return c ^ 0xffffffff
}

// trimChunk reduces the payload of a PNG chunk by n bytes and recomputes the
// length + CRC fields. n must be ≤ chunk payload length.
func trimChunk(chunk []byte, n int) []byte {
	if n <= 0 || len(chunk) < 12 {
		return chunk
	}
	// Parse: first 4 bytes = old length, next 4 = type, then payload, last 4 = CRC.
	oldLen := int(uint32(chunk[0])<<24 | uint32(chunk[1])<<16 | uint32(chunk[2])<<8 | uint32(chunk[3]))
	newLen := oldLen - n
	if newLen < 0 {
		newLen = 0
	}
	t := chunk[4:8]
	payload := chunk[8 : 8+newLen]
	// Rebuild length.
	chunk[0] = byte(newLen >> 24)
	chunk[1] = byte(newLen >> 16)
	chunk[2] = byte(newLen >> 8)
	chunk[3] = byte(newLen)
	// Recompute CRC.
	crc := pngCRC(append(append([]byte{}, t...), payload...))
	off := 8 + newLen
	chunk[off] = byte(crc >> 24)
	chunk[off+1] = byte(crc >> 16)
	chunk[off+2] = byte(crc >> 8)
	chunk[off+3] = byte(crc)
	return chunk[:8+newLen+4]
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
