// Package pack builds and verifies .zpack content packs (handoff §3). A pack is a zip
// of manifest.json + manifest.sig + content.sqlite + images/ (+ audio/). The builder
// enforces I6 (every story has a fully-provenanced, non-AI Commons image) and signs the
// manifest with ed25519; the verifier rejects unsigned, tampered, or hash-mismatched
// packs. (minisign-format compatibility is deferred to CP-02.)
package pack

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"sort"

	"github.com/parso/zhuwen-factory/internal/lexicon"
	"github.com/parso/zhuwen-factory/internal/minisign"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// SchemaVersion pins the frozen content.sqlite schema (handoff §3). Bump on any DDL change.
const SchemaVersion = 1

// Image is a Commons image record with full §8A provenance.
type Image struct {
	ID            string
	WordID        *int
	CanonID       string
	File          string // relative path inside the zip, e.g. images/img1@480.heic
	W, H          int
	License       string
	LicenseURL    string
	Author        string
	SourceURL     string
	RetrievedAt   string
	AICategorized bool   // I6: must be false
	Data          []byte // raw bytes; stub generated if nil (CP-01 fixture only)
}

// BodyToken is one entry of the segmented body stream (00 §9: {w, s, pn}).
type BodyToken struct {
	W       int    `json:"w"`             // word_id, or -1 for literal/proper noun
	Literal string `json:"lit,omitempty"` // surface text for literal/proper noun
	S       int    `json:"s"`             // sentence index
	PN      bool   `json:"pn,omitempty"`  // proper noun
}

// AlignToken is one word-level timing window (handoff §3 `alignment` table; §4.7). TokenIdx
// indexes into the story body stream; the app highlights the matching token during playback.
type AlignToken struct {
	TokenIdx int `json:"i"`
	T0ms     int `json:"t0"`
	T1ms     int `json:"t1"`
}

// Question is one comprehension MC question.
type Question struct {
	ID        string
	StoryID   string
	PromptZH  string
	Options   []string
	AnswerIdx int
	Band      string
}

// Story is a fully gated story ready to pack.
type Story struct {
	ID             string
	TitleZH        string
	TitleEN        string
	Band           string
	HSK3Level      int
	TokenCount     int
	TypeCount      int
	CoverageBitmap []byte
	NewTypeIDs     []int
	Topics         []string
	GrammarIDs     []string
	Body           []BodyToken
	CanonID        string
	Tier           string
	Origin         string
	SourceURLs     []string
	PDRationale    string
	CoverImageID   string
	Fixture        bool
	AudioFile      string       // relative path inside the zip, e.g. audio/s1.opus ("" = no audio)
	AudioData      []byte       // raw audio bytes; stub generated if nil (fixture only)
	Alignment      []AlignToken // word-level timings (FR-5.1); empty when AudioFile == ""
}

// Pack is the full set of pack inputs.
type Pack struct {
	ID               string
	Semver           string
	LexiconVersion   string
	CreatedAt        string
	Lexicon          *lexicon.Lexicon
	Stories          []Story
	Questions        []Question
	Images           []Image
	FoundationsCards []FoundationsCard

	// CP-09b audit fields (recorded in manifest.json, no SQLite change).
	AuditPassRate   float64
	AuditSampleSize int
	Generator       string
	Model           string
}

// FoundationsCard is one F0 card row (matches schema.sql foundations_card).
type FoundationsCard struct {
	WordID        int    // lexicon word_id (PK)
	ImageID       string // references image.id
	SetID         string // semantic set e.g. "animals"
	Stage         string // "F0"
	DistractorIDs []int  // JSON array of word_ids for multiple-choice
}

// Manifest is the signed pack manifest.
type Manifest struct {
	ID             string            `json:"id"`
	Semver         string            `json:"semver"`
	LexiconVersion string            `json:"lexicon_version"`
	CreatedAt      string            `json:"created_at"`
	SchemaVersion  int               `json:"schema_version"`
	Files          map[string]string `json:"files"` // path -> sha256 hex

	// CP-09b audit fields (handoff §6). Recorded by the audit stage and written into
	// manifest.json (no content.sqlite DDL change). The verifier rejects malformed/
	// out-of-range values (I4: a fabricated audit metric must not ship).
	AuditPassRate   float64 `json:"audit_pass_rate,omitempty"`
	AuditSampleSize int     `json:"audit_sample_size,omitempty"`
	Generator       string  `json:"generator,omitempty"` // e.g. "deepseek-rerank", "hand-authored"
	Model           string  `json:"model,omitempty"`     // e.g. "deepseek-chat", "none"
}

// StoryRef is a lightweight reference to a story in a pack, used for audit sampling.
type StoryRef struct {
	ID      string `json:"id"`
	TitleZH string `json:"title_zh"`
	Band    string `json:"band"`
}

// validateI6 enforces invariant I6 before any bytes are written.
func (p *Pack) validateI6() error {
	imgByID := map[string]Image{}
	for _, im := range p.Images {
		imgByID[im.ID] = im
	}
	for _, s := range p.Stories {
		if s.CoverImageID == "" {
			return fmt.Errorf("I6: story %q has no cover_image_id", s.ID)
		}
		im, ok := imgByID[s.CoverImageID]
		if !ok {
			return fmt.Errorf("I6: story %q references missing image %q", s.ID, s.CoverImageID)
		}
		if im.AICategorized {
			return fmt.Errorf("I6: image %q is AI-categorized", im.ID)
		}
		if im.License == "" || im.LicenseURL == "" || im.Author == "" || im.SourceURL == "" || im.RetrievedAt == "" {
			return fmt.Errorf("I6: image %q missing provenance record", im.ID)
		}
	}
	for _, fc := range p.FoundationsCards {
		if fc.ImageID == "" {
			return fmt.Errorf("I6: foundations_card word_id=%d has no image_id", fc.WordID)
		}
		im, ok := imgByID[fc.ImageID]
		if !ok {
			return fmt.Errorf("I6: foundations_card word_id=%d references missing image %q", fc.WordID, fc.ImageID)
		}
		if im.AICategorized {
			return fmt.Errorf("I6: foundations_card image %q is AI-categorized", im.ID)
		}
		if im.License == "" || im.LicenseURL == "" || im.Author == "" || im.SourceURL == "" || im.RetrievedAt == "" {
			return fmt.Errorf("I6: foundations_card image %q missing provenance record", im.ID)
		}
	}
	return nil
}

// Build writes a signed .zpack to outPath. priv signs the manifest.
func Build(p *Pack, outPath string, priv minisign.PrivateKey) error {
	if err := p.validateI6(); err != nil {
		return err
	}
	files, err := p.assembleFiles()
	if err != nil {
		return err
	}

	man := manifestFor(p, files)
	manBytes, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	// NFR-3/NFR-4 (CP-09c): a pack that breaches the size budget must not build. The
	// assertion is over the real assembled file set (audio + images + sqlite + manifest),
	// so an over-weight pack fails here rather than shipping.
	if err := CheckBudget(measureFiles(files, manBytes)); err != nil {
		return err
	}
	sig := minisign.Sign(priv, manBytes, fmt.Sprintf("pack %s v%s lexicon %s", p.ID, p.Semver, p.LexiconVersion))
	return writePack(outPath, files, manBytes, sig)
}

// assembleFiles builds the content file set (content.sqlite + image files + story audio) that
// goes into the zip, generating stub bytes for any nil image/audio (fixture tiers). Shared by
// Build and MeasureBudget so the size budget is measured over exactly what ships.
func (p *Pack) assembleFiles() (map[string][]byte, error) {
	dbBytes, err := p.buildSQLite()
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{"content.sqlite": dbBytes}
	for _, im := range p.Images {
		data := im.Data
		if data == nil {
			data = stubPNG(im.ID)
		}
		files[im.File] = data
	}
	// Story audio (I3: pregenerated in the factory). Stub bytes at fixture tiers; real
	// CosyVoice-rendered Opus replaces them at CP-09c. Hashed into the signed manifest.
	for _, s := range p.Stories {
		if s.AudioFile == "" {
			continue
		}
		data := s.AudioData
		if data == nil {
			data = []byte("ZHUWEN-FIXTURE-AUDIO:" + s.ID)
		}
		files[s.AudioFile] = data
	}
	return files, nil
}

// manifestFor computes the manifest (content-file hashes) for a pack.
func manifestFor(p *Pack, files map[string][]byte) Manifest {
	man := Manifest{
		ID:             p.ID,
		Semver:         p.Semver,
		LexiconVersion: p.LexiconVersion,
		CreatedAt:      p.CreatedAt,
		SchemaVersion:  SchemaVersion,
		Files:          map[string]string{},
		// CP-09b audit fields.
		AuditPassRate:   p.AuditPassRate,
		AuditSampleSize: p.AuditSampleSize,
		Generator:       p.Generator,
		Model:           p.Model,
	}
	for name, data := range files {
		sum := sha256.Sum256(data)
		man.Files[name] = hex.EncodeToString(sum[:])
	}
	return man
}

// writePack assembles the zip container deterministically.
func writePack(outPath string, files map[string][]byte, manBytes []byte, sig string) error {
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)

	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if err := writeZip(zw, n, files[n]); err != nil {
			zw.Close()
			return err
		}
	}
	if err := writeZip(zw, "manifest.json", manBytes); err != nil {
		zw.Close()
		return err
	}
	if err := writeZip(zw, "manifest.sig", []byte(sig)); err != nil {
		zw.Close()
		return err
	}
	return zw.Close()
}

func writeZip(zw *zip.Writer, name string, data []byte) error {
	// STORED (no compression): the container stays a standard zip while letting the iOS
	// side read entries with a tiny, dependency-free, on-device reader. File hashes and
	// the manifest signature are over file *contents*, so this is verification-neutral.
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// buildSQLite renders content.sqlite to a byte slice.
func (p *Pack) buildSQLite() ([]byte, error) {
	tmp, err := os.CreateTemp("", "zhuwen-*.sqlite")
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d;", SchemaVersion)); err != nil {
		db.Close()
		return nil, fmt.Errorf("user_version: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	if err := p.insertRows(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := db.Close(); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (p *Pack) insertRows(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	meta := [][2]string{
		{"schema_version", fmt.Sprintf("%d", SchemaVersion)},
		{"lexicon_version", p.LexiconVersion},
		{"pack_id", p.ID},
		{"semver", p.Semver},
	}
	for _, kv := range meta {
		if _, err := tx.Exec(`INSERT INTO meta(key,value) VALUES(?,?)`, kv[0], kv[1]); err != nil {
			return fmt.Errorf("insert meta %s: %w", kv[0], err)
		}
	}

	if p.Lexicon != nil {
		for _, w := range p.Lexicon.Words() {
			charIDs, _ := json.Marshal(w.CharIDs)
			if len(w.CharIDs) == 0 {
				charIDs = []byte("[]")
			}
			if _, err := tx.Exec(`INSERT INTO lexicon(word_id,simp,pinyin,hsk3_level,freq_rank,en,char_ids) VALUES(?,?,?,?,?,?,?)`,
				w.ID, w.Simp, w.Pinyin, w.HSK, w.FreqRank, w.En, string(charIDs)); err != nil {
				return fmt.Errorf("insert lexicon %d: %w", w.ID, err)
			}
		}
	}
	for _, im := range p.Images {
		var wid interface{}
		if im.WordID != nil {
			wid = *im.WordID
		}
		if _, err := tx.Exec(`INSERT INTO image(id,word_id,canon_id,file,w,h,license,license_url,author,source_url,retrieved_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			im.ID, wid, im.CanonID, im.File, im.W, im.H, im.License, im.LicenseURL, im.Author, im.SourceURL, im.RetrievedAt); err != nil {
			return fmt.Errorf("insert image %s: %w", im.ID, err)
		}
	}
	for _, s := range p.Stories {
		newTypes, _ := json.Marshal(s.NewTypeIDs)
		topics, _ := json.Marshal(s.Topics)
		grammar, _ := json.Marshal(s.GrammarIDs)
		sources, _ := json.Marshal(s.SourceURLs)
		body, _ := json.Marshal(s.Body)
		fix := 0
		if s.Fixture {
			fix = 1
		}
		var audioFile interface{}
		if s.AudioFile != "" {
			audioFile = s.AudioFile
		}
		var alignJSON interface{}
		if len(s.Alignment) > 0 {
			ab, _ := json.Marshal(s.Alignment)
			alignJSON = string(ab)
		}
		if _, err := tx.Exec(`INSERT INTO story(id,title_zh,title_en,band,hsk3_level,token_count,type_count,
			coverage_bitmap,new_type_ids,topics,grammar_ids,audio_file,alignment,body,canon_id,tier,origin,source_urls,pd_rationale,cover_image_id,fixture)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			s.ID, s.TitleZH, s.TitleEN, s.Band, s.HSK3Level, s.TokenCount, s.TypeCount,
			s.CoverageBitmap, string(newTypes), string(topics), string(grammar), audioFile, alignJSON, string(body),
			s.CanonID, s.Tier, s.Origin, string(sources), s.PDRationale, s.CoverImageID, fix); err != nil {
			return fmt.Errorf("insert story %s: %w", s.ID, err)
		}
		for _, a := range s.Alignment {
			if _, err := tx.Exec(`INSERT INTO alignment(story_id,token_idx,t0_ms,t1_ms) VALUES(?,?,?,?)`,
				s.ID, a.TokenIdx, a.T0ms, a.T1ms); err != nil {
				return fmt.Errorf("insert alignment %s#%d: %w", s.ID, a.TokenIdx, err)
			}
		}
	}
	for _, q := range p.Questions {
		opts, _ := json.Marshal(q.Options)
		if _, err := tx.Exec(`INSERT INTO question(id,story_id,prompt_zh,options,answer_idx,band) VALUES(?,?,?,?,?,?)`,
			q.ID, q.StoryID, q.PromptZH, string(opts), q.AnswerIdx, q.Band); err != nil {
			return fmt.Errorf("insert question %s: %w", q.ID, err)
		}
	}
	for _, fc := range p.FoundationsCards {
		dids, _ := json.Marshal(fc.DistractorIDs)
		if len(fc.DistractorIDs) == 0 {
			dids = []byte("[]")
		}
		if _, err := tx.Exec(`INSERT INTO foundations_card(word_id,image_id,set_id,stage,distractor_ids) VALUES(?,?,?,?,?)`,
			fc.WordID, fc.ImageID, fc.SetID, fc.Stage, string(dids)); err != nil {
			return fmt.Errorf("insert foundations_card %d: %w", fc.WordID, err)
		}
	}
	return tx.Commit()
}

// stubPNG generates a tiny valid 1×1 PNG whose pixel color is seeded from the given id,
// so every missing-image fallback is distinct and decodable on-device (UIImage supports
// PNG regardless of the .heic extension in the zip filename).
func stubPNG(id string) []byte {
	h := sha256.Sum256([]byte(id))
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: h[0], G: h[1], B: h[2], A: 255})
	var buf bytes.Buffer
	_ = png.Encode(&buf, img) // nil error for in-memory 1x1
	return buf.Bytes()
}
