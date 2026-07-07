package pack

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"os"

	_ "modernc.org/sqlite"
)

// PackInfo is a lightweight snapshot of a pack's story metadata, used for audit sampling
// without loading the full SQLite or running the I6 content checks.
type PackInfo struct {
	Stories []StoryRef
}

// ReadPackInfo opens a .zpack and reads story IDs/titles/bands from content.sqlite.
// It does not verify the signature or hashes (caller must Verify first).
func ReadPackInfo(zpackPath string) (*PackInfo, error) {
	zr, err := zip.OpenReader(zpackPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	var dbBytes []byte
	for _, f := range zr.File {
		if f.Name == "content.sqlite" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			dbBytes, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if dbBytes == nil {
		return nil, fmt.Errorf("info: content.sqlite not found in pack")
	}

	tmp, err := os.CreateTemp("", "zhuwen-info-*.sqlite")
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)
	if err := os.WriteFile(path, dbBytes, 0o600); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, title_zh, band FROM story ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("info query: %w", err)
	}
	defer rows.Close()

	info := &PackInfo{}
	for rows.Next() {
		var sr StoryRef
		if err := rows.Scan(&sr.ID, &sr.TitleZH, &sr.Band); err != nil {
			return nil, err
		}
		info.Stories = append(info.Stories, sr)
	}
	return info, rows.Err()
}
