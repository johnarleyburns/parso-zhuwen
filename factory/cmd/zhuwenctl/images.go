package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/parso/zhuwen-factory/internal/images"
	"github.com/parso/zhuwen-factory/internal/lexicon"
)

// cmdImages implements `zhuwenctl images ...` — the §8A Commons image pipeline:
// fetch (network, --live gated) → gate (hermetic) → curate (human decisions).
func cmdImages(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("images: expected subcommand: fetch | gate | curate")
	}
	switch args[0] {
	case "fetch":
		return cmdImagesFetch(args[1:])
	case "gate":
		return cmdImagesGate(args[1:])
	case "curate":
		return cmdImagesCurate(args[1:])
	default:
		return fmt.Errorf("images: unknown subcommand %q (expected: fetch | gate | curate)", args[0])
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
