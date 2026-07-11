package lexicon

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// lexiconSchema is the frozen lexicon.sqlite shape (§4.1, 00 §9 pack `lexicon` table slice).
// Stable integer word IDs are the primary key ("word IDs are forever"); level and freq-rank
// are attributes. char_ids is a JSON array. A one-row `meta` table pins the lexicon_version.
const lexiconSchema = `
CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE lexicon (
  word_id    INTEGER PRIMARY KEY,
  simp       TEXT NOT NULL UNIQUE,
  pinyin     TEXT NOT NULL,
  hsk3_level INTEGER NOT NULL,
  freq_rank  INTEGER NOT NULL,
  en         TEXT NOT NULL DEFAULT '',
  char_ids   TEXT NOT NULL
);
`

// WriteSQLite writes the lexicon to a fresh lexicon.sqlite at path (overwriting any existing
// file). The word IDs, forms, and attributes are frozen by lexicon_version so packs built
// against this version keep stable IDs.
func WriteSQLite(l *Lexicon, path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(lexiconSchema); err != nil {
		return fmt.Errorf("lexicon schema: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO meta(key,value) VALUES('lexicon_version',?)`, l.version); err != nil {
		tx.Rollback()
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO lexicon(word_id,simp,pinyin,hsk3_level,freq_rank,en,char_ids) VALUES(?,?,?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	for i := range l.words {
		w := &l.words[i]
		ids := w.CharIDs
		if ids == nil {
			ids = []int{}
		}
		cj, err := json.Marshal(ids)
		if err != nil {
			tx.Rollback()
			return err
		}
		if _, err := stmt.Exec(w.ID, w.Simp, w.Pinyin, w.HSK, w.FreqRank, w.En, string(cj)); err != nil {
			tx.Rollback()
			return fmt.Errorf("lexicon insert id %d: %w", w.ID, err)
		}
	}
	stmt.Close()
	return tx.Commit()
}

// ReadSQLite reads a lexicon.sqlite back into an immutable Lexicon.
func ReadSQLite(path string) (*Lexicon, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var version string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key='lexicon_version'`).Scan(&version); err != nil {
		return nil, fmt.Errorf("lexicon.sqlite: reading lexicon_version: %w", err)
	}
	rows, err := db.Query(`SELECT word_id,simp,pinyin,hsk3_level,freq_rank,en,char_ids FROM lexicon ORDER BY word_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var words []Word
	for rows.Next() {
		var w Word
		var cj string
		if err := rows.Scan(&w.ID, &w.Simp, &w.Pinyin, &w.HSK, &w.FreqRank, &w.En, &cj); err != nil {
			return nil, err
		}
		if cj != "" && cj != "[]" {
			if err := json.Unmarshal([]byte(cj), &w.CharIDs); err != nil {
				return nil, fmt.Errorf("lexicon.sqlite: char_ids for id %d: %w", w.ID, err)
			}
		}
		words = append(words, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return FromWords(version, words)
}

// IngestDir ingests one or more HSK-3.0 TSV files (same column format as Ingest) from a file
// or directory into a single Lexicon under lexicon_version. Directory files are read in sorted
// order and concatenated; word IDs and simplified forms must be unique across the whole set.
// The raw lists themselves are operator-supplied and license-cleared (see factory/data/README.md
// and plans/blockers.md B-1) — none are committed to the repo.
func IngestDir(src, version string) (*Lexicon, error) {
	info, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	var paths []string
	if info.IsDir() {
		entries, err := os.ReadDir(src)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(strings.ToLower(e.Name()), ".tsv") {
				paths = append(paths, filepath.Join(src, e.Name()))
			}
		}
		sort.Strings(paths)
		if len(paths) == 0 {
			return nil, fmt.Errorf("lexicon ingest: no .tsv files in %s", src)
		}
	} else {
		paths = []string{src}
	}

	var combined strings.Builder
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		combined.Write(b)
		combined.WriteString("\n")
	}
	return Ingest(strings.NewReader(combined.String()), version)
}
