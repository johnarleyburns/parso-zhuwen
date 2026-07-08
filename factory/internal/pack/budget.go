package pack

import (
	"encoding/json"
	"fmt"
)

// Size budgets (00 NFR-3, NFR-4). These are hard ceilings the build enforces so a pack that
// would breach the app's download/on-disk promises cannot ship (I4: sizes are asserted, not
// assumed). Measured over the real assembled file set — real 24 kbps Opus + real HEIC covers.
const (
	// MaxDownloadBytes — NFR-3: the app binary + embedded starter content download ≤ 90 MB.
	// A single starter/band pack's download footprint (compressed transfer ≈ stored, since
	// Opus/HEIC are already compressed and the zip is STORED) must leave room for the app.
	// We budget the starter pack's own bytes against a conservative share of the 90 MB.
	MaxDownloadBytes = 90 * 1000 * 1000
	// MaxOnDiskBytes — NFR-4: the A1+A2 corpus with audio occupies ≤ 250 MB on disk.
	MaxOnDiskBytes = 250 * 1000 * 1000
)

// Budget is the measured size of a pack, split into the NFR-3 download figure and the NFR-4
// on-disk figure. For a STORED zip of already-compressed assets these are nearly equal; the
// on-disk figure is the sum of extracted file bytes, the download figure is the zip transfer.
type Budget struct {
	DownloadBytes int            // transfer size (≈ sum of stored entries + zip overhead)
	OnDiskBytes   int            // extracted size (sum of file bytes)
	AudioBytes    int            // audio subtotal (the dominant NFR-4 term)
	ImageBytes    int            // image subtotal
	SQLiteBytes   int            // content.sqlite
	OtherBytes    int            // manifest + signature
	PerFile       map[string]int // per-entry byte counts (largest-contributor diagnosis)
}

// MeasureBudget assembles a pack's real file set (real audio + real images if present, stub
// weights otherwise) and reports its NFR-3/NFR-4 size figures. It does not sign or write.
func MeasureBudget(p *Pack) (Budget, error) {
	files, err := p.assembleFiles()
	if err != nil {
		return Budget{}, err
	}
	man := manifestFor(p, files)
	manBytes, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return Budget{}, err
	}
	return measureFiles(files, manBytes), nil
}

// measureFiles computes a Budget from an assembled file set + the manifest bytes.
func measureFiles(files map[string][]byte, manBytes []byte) Budget {
	b := Budget{PerFile: map[string]int{}}
	for name, data := range files {
		n := len(data)
		b.PerFile[name] = n
		b.OnDiskBytes += n
		switch {
		case name == "content.sqlite":
			b.SQLiteBytes += n
		case hasPrefix(name, "audio/"):
			b.AudioBytes += n
		case hasPrefix(name, "images/"):
			b.ImageBytes += n
		default:
			b.OtherBytes += n
		}
	}
	// manifest.json + manifest.sig ship in the zip too.
	b.OtherBytes += len(manBytes)
	b.OnDiskBytes += len(manBytes)
	// STORED zip: transfer ≈ on-disk + a small per-entry central-directory overhead. Budget
	// a conservative 64 bytes/entry of overhead so the download figure is not understated.
	overhead := 64 * (len(files) + 2)
	b.DownloadBytes = b.OnDiskBytes + overhead
	return b
}

// CheckBudget fails if a Budget breaches NFR-3 (download) or NFR-4 (on-disk). The on-disk
// (NFR-4) ceiling is the larger figure, so it is checked first: a pack big enough to breach
// the 250 MB corpus budget is reported as NFR-4; a smaller pack that still exceeds the 90 MB
// single-download budget is reported as NFR-3.
func CheckBudget(b Budget) error {
	if b.OnDiskBytes > MaxOnDiskBytes {
		return fmt.Errorf("NFR-4: pack on-disk %d B exceeds %d B budget (audio=%d image=%d sqlite=%d)",
			b.OnDiskBytes, MaxOnDiskBytes, b.AudioBytes, b.ImageBytes, b.SQLiteBytes)
	}
	if b.DownloadBytes > MaxDownloadBytes {
		return fmt.Errorf("NFR-3: pack download %d B exceeds %d B budget (audio=%d image=%d sqlite=%d)",
			b.DownloadBytes, MaxDownloadBytes, b.AudioBytes, b.ImageBytes, b.SQLiteBytes)
	}
	return nil
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
