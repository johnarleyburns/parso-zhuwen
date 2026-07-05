package pack

import (
	"archive/zip"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/parso/zhuwen-factory/internal/minisign"
	_ "modernc.org/sqlite"
)

// VerifyOptions tunes the reference verifier.
type VerifyOptions struct {
	// KnownLexiconVersions, if non-empty, is the set the app understands. A pack whose
	// lexicon_version is absent is rejected (handoff §3: "app refuses packs whose
	// lexicon_version it doesn't know").
	KnownLexiconVersions map[string]bool
	// SkipContentChecks disables the SQLite-level I6 audit (lower-level tests only).
	SkipContentChecks bool
}

// Verify opens a .zpack and enforces, in order: (1) minisign signature over manifest.json,
// (2) every manifest file hash, (3) lexicon_version acceptance, (4) I6 at the content
// level (every story has a fully-provenanced cover image). Returns the manifest on success.
func Verify(zpackPath string, pub minisign.PublicKey, opts *VerifyOptions) (*Manifest, error) {
	if opts == nil {
		opts = &VerifyOptions{}
	}
	zr, err := zip.OpenReader(zpackPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	contents := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		contents[f.Name] = data
	}

	manBytes, ok := contents["manifest.json"]
	if !ok {
		return nil, fmt.Errorf("verify: manifest.json missing")
	}
	sigBytes, ok := contents["manifest.sig"]
	if !ok {
		return nil, fmt.Errorf("verify: manifest.sig missing (unsigned pack)")
	}
	if err := minisign.Verify(pub, manBytes, string(sigBytes)); err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}

	var man Manifest
	if err := json.Unmarshal(manBytes, &man); err != nil {
		return nil, fmt.Errorf("verify: manifest parse: %w", err)
	}
	for name, want := range man.Files {
		data, ok := contents[name]
		if !ok {
			return nil, fmt.Errorf("verify: manifest lists %q but it is absent", name)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			return nil, fmt.Errorf("verify: hash mismatch for %q (tampered content)", name)
		}
	}

	if len(opts.KnownLexiconVersions) > 0 && !opts.KnownLexiconVersions[man.LexiconVersion] {
		return nil, fmt.Errorf("verify: unknown lexicon_version %q", man.LexiconVersion)
	}

	if !opts.SkipContentChecks {
		db, ok := contents["content.sqlite"]
		if !ok {
			return nil, fmt.Errorf("verify: content.sqlite missing")
		}
		if err := verifyI6(db); err != nil {
			return nil, err
		}
	}
	return &man, nil
}

// verifyI6 opens content.sqlite and enforces I6 at the row level: every story must carry
// a non-empty cover_image_id resolving to an image row with a complete provenance record.
func verifyI6(dbBytes []byte) error {
	tmp, err := os.CreateTemp("", "zhuwen-verify-*.sqlite")
	if err != nil {
		return err
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)
	if err := os.WriteFile(path, dbBytes, 0o600); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, cover_image_id FROM story`)
	if err != nil {
		return fmt.Errorf("verify I6: %w", err)
	}
	defer rows.Close()
	type ref struct{ story, image string }
	var refs []ref
	for rows.Next() {
		var storyID, imageID string
		if err := rows.Scan(&storyID, &imageID); err != nil {
			return err
		}
		refs = append(refs, ref{storyID, imageID})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range refs {
		if r.image == "" {
			return fmt.Errorf("verify I6: story %q has empty cover_image_id", r.story)
		}
		var license, licenseURL, author, sourceURL, retrievedAt string
		err := db.QueryRow(`SELECT license, license_url, author, source_url, retrieved_at FROM image WHERE id = ?`, r.image).
			Scan(&license, &licenseURL, &author, &sourceURL, &retrievedAt)
		if err == sql.ErrNoRows {
			return fmt.Errorf("verify I6: story %q references missing image %q", r.story, r.image)
		}
		if err != nil {
			return err
		}
		if license == "" || licenseURL == "" || author == "" || sourceURL == "" || retrievedAt == "" {
			return fmt.Errorf("verify I6: image %q missing provenance record", r.image)
		}
	}
	return nil
}
