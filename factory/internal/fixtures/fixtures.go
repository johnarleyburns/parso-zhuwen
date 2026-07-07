// Package fixtures builds the deterministic CP-01/CP-02 fixture pack and the golden
// negative variants (unsigned / tampered / imageless) that both the Go and Swift pack
// verifiers must reject. Shared by cmd/zhuwenctl (--devkey) and cmd/genfixtures.
package fixtures

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/parso/zhuwen-factory/internal/assets"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/gatevec"
	"github.com/parso/zhuwen-factory/internal/gen"
	"github.com/parso/zhuwen-factory/internal/lexicon"
	"github.com/parso/zhuwen-factory/internal/minisign"
	"github.com/parso/zhuwen-factory/internal/pack"
	"github.com/parso/zhuwen-factory/internal/pipeline"
	_ "modernc.org/sqlite"
)

// DevKey is the DEV-ONLY reproducible fixture-signing key (public seed on purpose; signs
// only vendored test fixtures, never production packs).
func DevKey() (minisign.PublicKey, minisign.PrivateKey) {
	var seed [32]byte
	var id [8]byte
	copy(seed[:], "zhuwen-cp02-devfixture-seed-0000")
	copy(id[:], "zhuwendv")
	return minisign.KeyFromSeed(seed, id)
}

// BuildFixturePack runs the pipeline over the embedded seeds and returns the pack.
func BuildFixturePack() (*pack.Pack, int, error) {
	lex, err := assets.Lexicon()
	if err != nil {
		return nil, 0, err
	}
	reg, err := assets.Canon()
	if err != nil {
		return nil, 0, err
	}
	band, err := pipeline.BuildFixtureBand(lex, assets.FrontierSimps())
	if err != nil {
		return nil, 0, err
	}
	res, err := pipeline.Run(pipeline.Config{
		Lexicon:  lex,
		Registry: reg,
		Band:     band,
		Provider: gen.NewFixtureProvider(lex, assets.FillerSimps()),
		GateCfg:  gate.DefaultConfig(),
	})
	if err != nil {
		return nil, 0, err
	}
	p := &pack.Pack{
		ID:             "a2",
		Semver:         "0.0.0",
		LexiconVersion: lex.Version(),
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		Lexicon:        lex,
		Stories:        res.Stories,
		Questions:      res.Questions,
		Images:         res.Images,
	}
	p.FoundationsCards = fixtureFoundationsCards(lex, res.Images)
	return p, len(res.Rejected), nil
}

// fixtureFoundationsCards synthesizes a small, deterministic set of F0 cards so the
// vendored pack exercises the Foundations reader + engine (CP-08a Part C). It reuses an
// existing provenanced image (satisfying I6) and draws distractors from same-set
// predecessors only (FR-11.3). Hermetic: no network, no Commons fetch.
func fixtureFoundationsCards(lex *lexicon.Lexicon, images []pack.Image) []pack.FoundationsCard {
	if len(images) == 0 {
		return nil
	}
	imageID := images[0].ID
	var singles []lexicon.Word
	for _, w := range lex.Words() {
		if len([]rune(w.Simp)) == 1 {
			singles = append(singles, w)
		}
		if len(singles) == 6 {
			break
		}
	}
	var cards []pack.FoundationsCard
	var taught []int
	for _, w := range singles {
		distractors := append([]int(nil), taught...)
		if len(distractors) > 3 {
			distractors = distractors[len(distractors)-3:]
		}
		cards = append(cards, pack.FoundationsCard{
			WordID:        w.ID,
			ImageID:       imageID,
			SetID:         "foundations-demo",
			Stage:         "F0",
			DistractorIDs: distractors,
		})
		taught = append(taught, w.ID)
	}
	return cards
}

// WriteAll writes the positive pack, its pubkey, and the three golden negatives into dir.
func WriteAll(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	pub, priv := DevKey()
	p, _, err := BuildFixturePack()
	if err != nil {
		return err
	}

	positive := filepath.Join(dir, "fixture-a2-v0.zpack")
	if err := pack.Build(p, positive, priv); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "zhuwen-dev.pub"), []byte(pub.Encode()), 0o644); err != nil {
		return err
	}

	files, err := readZip(positive)
	if err != nil {
		return err
	}

	// (a) unsigned: drop manifest.sig
	unsigned := cloneExcept(files, "manifest.sig")
	if err := writeStoredZip(filepath.Join(dir, "golden-unsigned.zpack"), unsigned); err != nil {
		return err
	}

	// (b) tampered: flip a byte of content.sqlite, keep manifest as-is
	tampered := cloneExcept(files)
	tampered["content.sqlite"] = append(append([]byte(nil), files["content.sqlite"]...), 0x00)
	if err := writeStoredZip(filepath.Join(dir, "golden-tampered.zpack"), tampered); err != nil {
		return err
	}

	// (c) imageless: null cover_image_id, then RE-SIGN so only I6 fails
	imageless := cloneExcept(files, "manifest.json", "manifest.sig")
	nulled, err := nullOutCover(files["content.sqlite"])
	if err != nil {
		return err
	}
	imageless["content.sqlite"] = nulled
	man := manifestFor(p, imageless)
	manBytes, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	sig := minisign.Sign(priv, manBytes, fmt.Sprintf("pack %s v%s lexicon %s", p.ID, p.Semver, p.LexiconVersion))
	imageless["manifest.json"] = manBytes
	imageless["manifest.sig"] = []byte(sig)
	if err := writeStoredZip(filepath.Join(dir, "golden-imageless.zpack"), imageless); err != nil {
		return err
	}

	// CP-04: shared Go/Swift coverage-gate vector suite (handoff §7).
	if err := gatevec.WriteJSON(dir); err != nil {
		return err
	}
	return nil
}

func manifestFor(p *pack.Pack, files map[string][]byte) pack.Manifest {
	man := pack.Manifest{
		ID: p.ID, Semver: p.Semver, LexiconVersion: p.LexiconVersion,
		CreatedAt: p.CreatedAt, SchemaVersion: pack.SchemaVersion, Files: map[string]string{},
	}
	for name, data := range files {
		if name == "manifest.json" || name == "manifest.sig" {
			continue
		}
		sum := sha256.Sum256(data)
		man.Files[name] = hex.EncodeToString(sum[:])
	}
	return man
}

func nullOutCover(dbBytes []byte) ([]byte, error) {
	tmp, err := os.CreateTemp("", "zhuwen-imageless-*.sqlite")
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
	if _, err := db.Exec(`UPDATE story SET cover_image_id = ''`); err != nil {
		db.Close()
		return nil, err
	}
	if err := db.Close(); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func readZip(path string) (map[string][]byte, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	out := map[string][]byte{}
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
		out[f.Name] = data
	}
	return out, nil
}

func cloneExcept(files map[string][]byte, drop ...string) map[string][]byte {
	skip := map[string]bool{}
	for _, d := range drop {
		skip[d] = true
	}
	out := map[string][]byte{}
	for k, v := range files {
		if skip[k] {
			continue
		}
		out[k] = append([]byte(nil), v...)
	}
	return out
}

func writeStoredZip(path string, files map[string][]byte) error {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			zw.Close()
			return err
		}
		if _, err := w.Write(data); err != nil {
			zw.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
