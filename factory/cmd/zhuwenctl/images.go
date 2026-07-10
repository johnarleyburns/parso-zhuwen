package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/parso/zhuwen-factory/internal/images"
	"github.com/parso/zhuwen-factory/internal/lexicon"
	"github.com/parso/zhuwen-factory/internal/pack"
)

// cmdImages implements `zhuwenctl images ...` — the §8A Commons image pipeline:
// fetch (network, --live gated) → gate (hermetic) → curate (human decisions) →
// process (HEIC encode, external), followed by join into the pack build.
func cmdImages(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("images: expected subcommand: fetch | gate | curate | curate-canon | process")
	}
	switch args[0] {
	case "fetch":
		return cmdImagesFetch(args[1:])
	case "gate":
		return cmdImagesGate(args[1:])
	case "curate":
		return cmdImagesCurate(args[1:])
	case "curate-canon":
		return cmdImagesCurateCanon(args[1:])
	case "process":
		return cmdImagesProcess(args[1:])
	default:
		return fmt.Errorf("images: unknown subcommand %q (expected: fetch | gate | curate | curate-canon | process)", args[0])
	}
}

func cmdImagesFetch(args []string) error {
	inventory := flagValue(args, "--inventory")
	out := flagValue(args, "--out")
	live := hasFlag(args, "--live")
	n := 6
	if v := flagValue(args, "--n"); v != "" {
		fmt.Sscanf(v, "%d", &n)
	}
	if inventory == "" || out == "" {
		return fmt.Errorf("images fetch: --inventory <tsv> and --out <dir> required")
	}
	if !live {
		return fmt.Errorf("images fetch: --live required (Commons is anonymous, no secret, but network is gated for hermetic CI)")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	b, err := os.ReadFile(inventory)
	if err != nil {
		return fmt.Errorf("inventory: %w", err)
	}
	seeds := parseInventoryTSV(string(b))
	if len(seeds) == 0 {
		return fmt.Errorf("inventory: no seeds loaded from %s", inventory)
	}
	fmt.Printf("images fetch: %d words from %s\n", len(seeds), inventory)

	fc := &images.FetchClient{}
	var results []fetchResult
	for i, s := range seeds {
		fmt.Printf("  [%d/%d] %s (%s)\n", i+1, len(seeds), s.Simp, s.En)
		cands, err := fc.FetchCandidates(s.En, s.Simp, n, 3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: %v\n", s.Simp, err)
			continue
		}
		best, alts, rejects, _ := images.GateCandidates(cands)
		var gateRejects []images.GateReject
		for _, r := range rejects {
			gateRejects = append(gateRejects, r)
		}
		prov := make(map[string]images.Provenance)
		for _, c := range cands {
			prov[c.Title] = images.Provenance{
				File:        c.Title,
				License:     c.License,
				LicenseURL:  c.LicenseURL,
				Author:      c.Author,
				SourceURL:   c.DescURL,
				RetrievedAt: c.DescURL,
				W:           c.W,
				H:           c.H,
			}
		}
		results = append(results, fetchResult{
			Simp:    s.Simp,
			En:      s.En,
			Set:     s.Set,
			Best:    best,
			Alts:    alts,
			Rejects: gateRejects,
			Prov:    prov,
		})
	}
	candsPath := out + "/candidates.json"
	writeJSON(candsPath, results)
	fmt.Printf("wrote %d word results → %s\n", len(results), candsPath)
	return nil
}

type fetchResult struct {
	Simp    string                       `json:"simp"`
	En      string                       `json:"en"`
	Set     string                       `json:"set"`
	Best    *images.Candidate            `json:"best,omitempty"`
	Alts    []images.Candidate           `json:"alternates,omitempty"`
	Rejects []images.GateReject          `json:"rejected,omitempty"`
	Prov    map[string]images.Provenance `json:"provenance,omitempty"`
}

func cmdImagesGate(args []string) error {
	candidatesPath := flagValue(args, "--candidates")
	out := flagValue(args, "--out")
	if candidatesPath == "" || out == "" {
		return fmt.Errorf("images gate: --candidates <json> and --out <json> required")
	}
	var results []fetchResult
	if err := readJSON(candidatesPath, &results); err != nil {
		return err
	}
	for i := range results {
		var cands []images.Candidate
		if results[i].Best != nil {
			cands = append(cands, *results[i].Best)
		}
		cands = append(cands, results[i].Alts...)
		best, alts, rejects, _ := images.GateCandidates(cands)
		results[i].Best = best
		results[i].Alts = alts
		results[i].Rejects = rejects
	}
	writeJSON(out, results)
	fmt.Printf("gated %d word results → %s\n", len(results), out)
	return nil
}

func cmdImagesCurate(args []string) error {
	decisionsPath := flagValue(args, "--decisions")
	lexiconPath := flagValue(args, "--lexicon")
	out := flagValue(args, "--out")
	if decisionsPath == "" || lexiconPath == "" || out == "" {
		return fmt.Errorf("images curate: --decisions <json> --lexicon <sqlite> --out <json> required")
	}
	decisions, err := images.LoadDecisions(decisionsPath)
	if err != nil {
		return fmt.Errorf("load decisions: %w", err)
	}
	fmt.Printf("loaded %d decisions\n", len(decisions))

	lex, err := lexicon.ReadSQLite(lexiconPath)
	if err != nil {
		return fmt.Errorf("lexicon: %w", err)
	}

	// Build wordID map from lexicon.
	wordIDMap := make(map[string]int, len(decisions))
	for _, d := range decisions {
		w, ok := lex.LookupSimp(d.WordKey())
		if !ok {
			fmt.Fprintf(os.Stderr, "  WARNING: word %q not in lexicon, skipping\n", d.WordKey())
			continue
		}
		wordIDMap[d.WordKey()] = w.ID
	}

	// For now, curate produces an image record list as JSON.
	// In production, `process` + `join` would consume this.
	type curatedImage struct {
		ID           string `json:"id"`
		WordID       int    `json:"word_id"`
		Simp         string `json:"simp"`
		File         string `json:"file"`
		CommonsTitle string `json:"commons_title"`
		License      string `json:"license"`
		LicenseURL   string `json:"license_url"`
		Author       string `json:"author"`
		SourceURL    string `json:"source_url"`
		RetrievedAt  string `json:"retrieved_at"`
		W            int    `json:"w,omitempty"`
		H            int    `json:"h,omitempty"`
	}

	var curated []curatedImage
	for _, d := range decisions {
		wid, ok := wordIDMap[d.WordKey()]
		if !ok {
			continue
		}
		title := d.CommonsTitle()
		curated = append(curated, curatedImage{
			ID:           fmt.Sprintf("img-foundations-%s", d.WordKey()),
			WordID:       wid,
			Simp:         d.WordKey(),
			File:         fmt.Sprintf("images/%s@480.heic", fmt.Sprintf("img-foundations-%s", d.WordKey())),
			CommonsTitle: title,
			SourceURL:    d.CommonsPageURL(),
		})
	}
	writeJSON(out, curated)
	fmt.Printf("curated %d images → %s\n", len(curated), out)
	return nil
}

// cmdImagesCurateCanon curates canon story covers (CP-09c Part D). Unlike `curate` (keyed by
// lexicon word_id for Foundations), this keys decisions by title_zh → canon_id via the cover
// inventory TSV, fetches each chosen file's provenance from Commons (--live), and emits
// signed-off pack.Image records (CanonID set) ready for the pipeline join stage.
//
// The reviewer's manual per-image license verification (FR-11.2/I4) is recorded via
// --signed-off: the flag asserts the owner has verified every chosen file's license on its
// Commons page. Without it, the ship-readiness sign-off gate is not satisfied.
func cmdImagesCurateCanon(args []string) error {
	decisionsPath := flagValue(args, "--decisions")
	inventoryPath := flagValue(args, "--inventory")
	out := flagValue(args, "--out")
	live := hasFlag(args, "--live")
	signedOff := hasFlag(args, "--signed-off")
	if decisionsPath == "" || inventoryPath == "" || out == "" {
		return fmt.Errorf("images curate-canon: --decisions <json> --inventory <tsv> --out <json> required")
	}

	decisions, err := images.LoadDecisions(decisionsPath)
	if err != nil {
		return fmt.Errorf("load decisions: %w", err)
	}

	canonIDMap, err := loadCanonIDMap(inventoryPath)
	if err != nil {
		return err
	}
	fmt.Printf("loaded %d decisions, %d canon inventory rows\n", len(decisions), len(canonIDMap))

	var titles []string
	seen := map[string]bool{}
	for _, d := range decisions {
		if d.Decision == "" || d.Decision == "__reject__" {
			continue
		}
		t := d.CommonsTitle()
		if !seen[t] {
			seen[t] = true
			titles = append(titles, t)
		}
	}

	if !live {
		return fmt.Errorf("images curate-canon: --live required to fetch provenance from Commons (anonymous GET, no secret; gated for hermetic CI, I2)")
	}
	fc := &images.FetchClient{}
	fmt.Printf("fetching provenance for %d Commons files…\n", len(titles))
	prov, err := fc.FetchProvenanceByTitles(titles)
	if err != nil {
		return fmt.Errorf("fetch provenance: %w", err)
	}

	if signedOff {
		for k, p := range prov {
			p.SignedOff = true
			p.SignedBy = "owner"
			p.SignedAt = time.Now().UTC().Format("2006-01-02")
			prov[k] = p
		}
	}

	imgs, err := images.CanonDecisionsToImages(decisions, prov, canonIDMap, signedOff)
	if err != nil {
		return err
	}

	writeJSON(out, imgs)
	fmt.Printf("curated %d canon covers → %s (signed-off=%v)\n", len(imgs), out, signedOff)
	return nil
}

// loadCanonIDMap reads the cover inventory TSV and returns title_zh → canon_id.
func loadCanonIDMap(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("inventory: %w", err)
	}
	m := map[string]string{}
	for _, line := range splitLines(string(b)) {
		if line == "" || line[0] == '#' {
			continue
		}
		f := splitTabs(line)
		if len(f) < 2 {
			continue
		}
		m[f[0]] = f[1]
	}
	return m, nil
}

// cmdImagesProcess implements `zhuwenctl images process` — the §8A HEIC encode stage
// (handoff §1 external-build-time-stage pattern, like tts.ModeReal). In stub mode
// (default, no --live) it emits deterministic marker bytes suitable for CI. In real mode
// (--live) it shells out to a Python image-processing script that downloads each Commons
// file, resizes to the target size, and HEIC-encodes it, populating pack.Image.Data.
func cmdImagesProcess(args []string) error {
	in := flagValue(args, "--in")
	out := flagValue(args, "--out")
	live := hasFlag(args, "--live")
	targetPX := 480
	if v := flagValue(args, "--target-px"); v != "" {
		fmt.Sscanf(v, "%d", &targetPX)
	}
	if in == "" || out == "" {
		return fmt.Errorf("images process: --in <json> and --out <json> required")
	}

	var imgs []pack.Image
	if err := readJSON(in, &imgs); err != nil {
		return fmt.Errorf("images process: read input: %w", err)
	}

	var cfg images.EncodeConfig
	if live {
		pythonBin := flagValue(args, "--python")
		script := flagValue(args, "--script")
		if pythonBin == "" || script == "" {
			return fmt.Errorf("images process: --live requires --python <bin> and --script <path>")
		}
		cfg = images.EncodeConfig{
			Mode:      images.EncodeModeReal,
			TargetPX:  targetPX,
			PythonBin: pythonBin,
			Script:    script,
			WorkDir:   flagValue(args, "--work-dir"),
		}
	} else {
		cfg = images.DefaultEncodeConfig()
		cfg.TargetPX = targetPX
	}
	cfg.RepresentativeStub = hasFlag(args, "--representative-stub")

	encoded, err := images.EncodePackImages(imgs, cfg)
	if err != nil {
		return fmt.Errorf("images process: %w", err)
	}

	// Null out Data for the JSON output; the encode writes a sidecar raw blob so
	// the JSON stays readable. In copy-only mode the tool writes a sidecar dir.
	if hasFlag(args, "--sidecar") {
		sidecar := out + ".d"
		os.MkdirAll(sidecar, 0o755)
		for i, im := range encoded {
			heicPath := sidecar + "/" + im.File
			os.MkdirAll(filepath.Dir(heicPath), 0o755)
			if im.Data != nil {
				os.WriteFile(heicPath, im.Data, 0o644)
			}
			encoded[i].Data = nil
		}
		fmt.Printf("encoded %d images, sidecar HEIC blobs → %s/\n", len(encoded), sidecar)
	}

	writeJSON(out, encoded)
	fmt.Printf("wrote %d encoded images → %s (mode=%s, target=%dpx)\n", len(encoded), out, cfg.Mode, targetPX)
	return nil
}

func writeJSON(path string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	os.WriteFile(path, b, 0o644)
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// seed is a word entry parsed from a TSV inventory.
type seed struct{ Simp, Pinyin, En, Set string }

func parseInventoryTSV(text string) []seed {
	var seeds []seed
	for _, line := range splitLines(text) {
		if line == "" || line[0] == '#' {
			continue
		}
		f := splitTabs(line)
		if len(f) < 3 {
			continue
		}
		s := seed{Simp: f[0], Pinyin: f[1], En: f[2]}
		if len(f) > 3 {
			s.Set = f[3]
		}
		seeds = append(seeds, s)
	}
	return seeds
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitTabs(s string) []string {
	var f []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\t' {
			f = append(f, s[start:i])
			start = i + 1
		}
	}
	f = append(f, s[start:])
	return f
}
