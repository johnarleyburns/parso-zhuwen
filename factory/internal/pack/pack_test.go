package pack

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parso/zhuwen-factory/internal/minisign"
)

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func intp(i int) *int { return &i }

func validImage() Image {
	return Image{
		ID: "img1", CanonID: "C5-x", File: "images/img1@480.heic", W: 480, H: 480,
		License: "PD", LicenseURL: "https://creativecommons.org/publicdomain/mark/1.0/",
		Author: "Commons", SourceURL: "https://zh.wikipedia.org/wiki/X", RetrievedAt: "2026-07-04",
	}
}

func validStory() Story {
	return Story{
		ID: "s1", TitleZH: "甲", Band: "A2", HSK3Level: 2, TokenCount: 200, TypeCount: 2,
		CoverageBitmap: []byte{0x12}, NewTypeIDs: []int{11}, Origin: "canon",
		SourceURLs: []string{"u"}, PDRationale: "PD", CoverImageID: "img1",
		Body: []BodyToken{{W: 1, S: 0}}, Fixture: true,
	}
}

func validPack() *Pack {
	return &Pack{
		ID: "a2", Semver: "0.0.0", LexiconVersion: "v0", CreatedAt: "2026-07-04T00:00:00Z",
		Stories:   []Story{validStory()},
		Images:    []Image{validImage()},
		Questions: []Question{{ID: "s1-q1", StoryID: "s1", PromptZH: "?", Options: []string{"a", "b"}, AnswerIdx: 0, Band: "A2"}},
	}
}

// devKey is a deterministic dev/test key (fixed seed -> stable pubkey). Fixtures only.
func devKey(t *testing.T) (minisign.PublicKey, minisign.PrivateKey) {
	t.Helper()
	var seed [32]byte
	var id [8]byte
	copy(seed[:], "zhuwen-cp02-golden-seed-00000000")
	copy(id[:], "zhuwengd")
	return minisign.KeyFromSeed(seed, id)
}

func buildTemp(t *testing.T, p *Pack) (string, minisign.PublicKey) {
	t.Helper()
	pub, priv := devKey(t)
	out := filepath.Join(t.TempDir(), "pack.zpack")
	if err := Build(p, out, priv); err != nil {
		t.Fatalf("build: %v", err)
	}
	return out, pub
}

func mustPriv(t *testing.T) minisign.PrivateKey {
	t.Helper()
	_, priv := devKey(t)
	return priv
}

func TestBuildVerifyRoundTrip(t *testing.T) {
	out, pub := buildTemp(t, validPack())
	man, err := Verify(out, pub, nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if man.ID != "a2" || man.LexiconVersion != "v0" {
		t.Errorf("manifest = %+v", man)
	}
	if man.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version = %d, want %d", man.SchemaVersion, SchemaVersion)
	}
	if _, ok := man.Files["content.sqlite"]; !ok {
		t.Error("manifest missing content.sqlite hash")
	}
	if _, ok := man.Files["images/img1@480.heic"]; !ok {
		t.Error("manifest missing image hash")
	}
}

func TestI6ImagelessStoryFailsBuild(t *testing.T) {
	p := validPack()
	p.Stories[0].CoverImageID = "" // I6 violation
	err := Build(p, filepath.Join(t.TempDir(), "x.zpack"), mustPriv(t))
	if err == nil || !strings.Contains(err.Error(), "I6") {
		t.Fatalf("expected I6 failure, got %v", err)
	}
}

func TestI6MissingImageReferenceFailsBuild(t *testing.T) {
	p := validPack()
	p.Stories[0].CoverImageID = "ghost"
	err := Build(p, filepath.Join(t.TempDir(), "x.zpack"), mustPriv(t))
	if err == nil || !strings.Contains(err.Error(), "I6") {
		t.Fatalf("expected I6 missing-image failure, got %v", err)
	}
}

func TestI6IncompleteProvenanceFailsBuild(t *testing.T) {
	p := validPack()
	p.Images[0].Author = "" // provenance incomplete
	err := Build(p, filepath.Join(t.TempDir(), "x.zpack"), mustPriv(t))
	if err == nil || !strings.Contains(err.Error(), "provenance") {
		t.Fatalf("expected provenance failure, got %v", err)
	}
}

func TestI6AICategorizedFailsBuild(t *testing.T) {
	p := validPack()
	p.Images[0].AICategorized = true
	err := Build(p, filepath.Join(t.TempDir(), "x.zpack"), mustPriv(t))
	if err == nil || !strings.Contains(err.Error(), "AI-categorized") {
		t.Fatalf("expected AI-categorized failure, got %v", err)
	}
}

func TestI6FoundationsCardMissingImage(t *testing.T) {
	p := validPack()
	p.FoundationsCards = []FoundationsCard{
		{WordID: 1, ImageID: "img-foundations-dog", SetID: "animals", Stage: "F0"},
	}
	err := Build(p, filepath.Join(t.TempDir(), "x.zpack"), mustPriv(t))
	if err == nil || !strings.Contains(err.Error(), "missing image") {
		t.Fatalf("expected foundations_card missing-image failure, got %v", err)
	}
}

func TestI6FoundationsCardEmptyImageID(t *testing.T) {
	p := validPack()
	p.FoundationsCards = []FoundationsCard{
		{WordID: 1, ImageID: "", SetID: "animals", Stage: "F0"},
	}
	err := Build(p, filepath.Join(t.TempDir(), "x.zpack"), mustPriv(t))
	if err == nil || !strings.Contains(err.Error(), "no image_id") {
		t.Fatalf("expected foundations_card no-image_id failure, got %v", err)
	}
}

func TestFoundationsCardRoundTrip(t *testing.T) {
	p := validPack()
	fcImage := validImage()
	fcImage.ID = "img-foundations-dog"
	p.Images = append(p.Images, fcImage)
	p.FoundationsCards = []FoundationsCard{
		{WordID: 101, ImageID: "img-foundations-dog", SetID: "animals", Stage: "F0", DistractorIDs: []int{102, 103, 104}},
	}
	p.Stories = nil

	out, pub := buildTemp(t, p)
	man, err := Verify(out, pub, nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, ok := man.Files["content.sqlite"]; !ok {
		t.Error("manifest missing content.sqlite hash")
	}

	dbBytes := entryBytes(t, out, "content.sqlite")
	tmp := filepath.Join(t.TempDir(), "fc.sqlite")
	if err := os.WriteFile(tmp, dbBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var wordID int
	var imageID, setID, stage string
	var distractorIDs string
	if err := db.QueryRow(`SELECT word_id, image_id, set_id, stage, distractor_ids FROM foundations_card WHERE word_id = 101`).Scan(&wordID, &imageID, &setID, &stage, &distractorIDs); err != nil {
		t.Fatal(err)
	}
	if wordID != 101 || imageID != "img-foundations-dog" || setID != "animals" || stage != "F0" {
		t.Fatalf("row = word_id=%d image=%q set=%q stage=%q", wordID, imageID, setID, stage)
	}
	var ids []int
	if err := json.Unmarshal([]byte(distractorIDs), &ids); err != nil {
		t.Fatalf("distractor_ids json: %v", err)
	}
	if len(ids) != 3 || ids[0] != 102 || ids[2] != 104 {
		t.Fatalf("distractor_ids = %v", ids)
	}
}

// --- Golden negative suite (handoff §6 CP-02): unsigned / tampered / imageless ---

func TestGoldenUnsignedRejected(t *testing.T) {
	out, pub := buildTemp(t, validPack())
	unsigned := rezip(t, out, func(name string, data []byte) ([]byte, bool) {
		return data, name != "manifest.sig"
	})
	if _, err := Verify(unsigned, pub, nil); err == nil || !strings.Contains(err.Error(), "manifest.sig missing") {
		t.Fatalf("expected unsigned rejection, got %v", err)
	}
}

func TestGoldenTamperedContentRejected(t *testing.T) {
	out, pub := buildTemp(t, validPack())
	tampered := rezip(t, out, func(name string, data []byte) ([]byte, bool) {
		if name == "content.sqlite" {
			return append(data, 0x00), true
		}
		return data, true
	})
	if _, err := Verify(tampered, pub, nil); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("expected hash-mismatch rejection, got %v", err)
	}
}

// TestGoldenImagelessRejected proves the VERIFIER (not just the builder) enforces I6:
// a validly-signed pack whose SQLite has been rewritten to null out cover_image_id must
// still be rejected on the content-level I6 audit.
func TestGoldenImagelessRejected(t *testing.T) {
	pub, priv := devKey(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "imageless.zpack")
	if err := Build(validPack(), out, priv); err != nil {
		t.Fatal(err)
	}

	imageless := rezip(t, out, func(name string, data []byte) ([]byte, bool) {
		if name == "content.sqlite" {
			return nullOutCover(t, data), true
		}
		return data, true
	})
	// Re-sign so signature + hashes are valid; only I6 should fail.
	resigned := resignPack(t, imageless, priv)

	if _, err := Verify(resigned, pub, nil); err == nil || !strings.Contains(err.Error(), "I6") {
		t.Fatalf("expected I6 content rejection, got %v", err)
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	out, _ := buildTemp(t, validPack())
	otherPub, _, _ := minisign.GenerateKey()
	if _, err := Verify(out, otherPub, nil); err == nil {
		t.Fatal("expected signature failure with wrong key")
	}
}

func TestVerifyLexiconVersionAcceptance(t *testing.T) {
	out, pub := buildTemp(t, validPack())
	// Accepted set includes "v0".
	if _, err := Verify(out, pub, &VerifyOptions{KnownLexiconVersions: map[string]bool{"v0": true}}); err != nil {
		t.Fatalf("v0 should be accepted: %v", err)
	}
	// Unknown lexicon version rejected.
	_, err := Verify(out, pub, &VerifyOptions{KnownLexiconVersions: map[string]bool{"v9": true}})
	if err == nil || !strings.Contains(err.Error(), "unknown lexicon_version") {
		t.Fatalf("expected unknown-lexicon rejection, got %v", err)
	}
}

// TestAudioAndAlignmentRoundTrip proves CP-06 pack plumbing: a story with audio + word-level
// timings ships an audio entry (hashed in the manifest), the story.audio_file/alignment
// columns, and one alignment row per token (handoff §3, §4.7, FR-5.1).
func TestAudioAndAlignmentRoundTrip(t *testing.T) {
	p := validPack()
	p.Stories[0].AudioFile = "audio/s1.opus"
	p.Stories[0].Alignment = []AlignToken{{TokenIdx: 0, T0ms: 250, T1ms: 510}}
	out, pub := buildTemp(t, p)

	man, err := Verify(out, pub, nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, ok := man.Files["audio/s1.opus"]; !ok {
		t.Fatal("manifest missing audio hash")
	}

	// Inspect the extracted SQLite: audio_file column + alignment table.
	dbBytes := entryBytes(t, out, "content.sqlite")
	tmp := filepath.Join(t.TempDir(), "c.sqlite")
	if err := os.WriteFile(tmp, dbBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var audioFile string
	var alignJSON string
	if err := db.QueryRow(`SELECT audio_file, alignment FROM story WHERE id='s1'`).Scan(&audioFile, &alignJSON); err != nil {
		t.Fatal(err)
	}
	if audioFile != "audio/s1.opus" {
		t.Fatalf("audio_file = %q", audioFile)
	}
	var decoded []AlignToken
	if err := json.Unmarshal([]byte(alignJSON), &decoded); err != nil {
		t.Fatalf("alignment json: %v", err)
	}
	if len(decoded) != 1 || decoded[0].T1ms != 510 {
		t.Fatalf("story.alignment = %+v", decoded)
	}

	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM alignment WHERE story_id='s1'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("alignment rows = %d, want 1", rows)
	}
}

// --- helpers ---

// entryBytes reads one entry's bytes from a zpack.
func entryBytes(t *testing.T, path, name string) []byte {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == name {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			return data
		}
	}
	t.Fatalf("entry %q not found", name)
	return nil
}

// nullOutCover rewrites content.sqlite setting every story.cover_image_id to ”.
func nullOutCover(t *testing.T, dbBytes []byte) []byte {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "mut.sqlite")
	if err := os.WriteFile(tmp, dbBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE story SET cover_image_id = ''`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	out, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// resignPack recomputes the manifest hashes over the current content and re-signs it.
func resignPack(t *testing.T, src string, priv minisign.PrivateKey) string {
	t.Helper()
	files := map[string][]byte{}
	zr, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == "manifest.json" || f.Name == "manifest.sig" {
			continue
		}
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = data
	}
	zr.Close()

	p := &Pack{ID: "a2", Semver: "0.0.0", LexiconVersion: "v0", CreatedAt: "2026-07-04T00:00:00Z"}
	man := manifestFor(p, files)
	manBytes := mustJSON(t, man)
	sig := minisign.Sign(priv, manBytes, "pack a2 v0.0.0 lexicon v0")
	dst := filepath.Join(t.TempDir(), "resigned.zpack")
	if err := writePack(dst, files, manBytes, sig); err != nil {
		t.Fatal(err)
	}
	return dst
}

// rezip copies a zip, applying mutate to each entry (keep=false drops it).
func rezip(t *testing.T, src string, mutate func(name string, data []byte) ([]byte, bool)) string {
	t.Helper()
	zr, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		nd, keep := mutate(f.Name, data)
		if !keep {
			continue
		}
		w, _ := zw.Create(f.Name)
		w.Write(nd)
	}
	zw.Close()
	dst := filepath.Join(t.TempDir(), "re.zpack")
	if err := os.WriteFile(dst, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}
