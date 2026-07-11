package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/parso/zhuwen-factory/internal/assets"
	"github.com/parso/zhuwen-factory/internal/fixtures"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/gen"
	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/images"
	"github.com/parso/zhuwen-factory/internal/lexicon"
	"github.com/parso/zhuwen-factory/internal/pack"
	"github.com/parso/zhuwen-factory/internal/pipeline"
)

// cmdAppPack implements `zhuwenctl app-pack`: the full real-content app-pack build
// path (Workstream D). It loads the real HSK-3.0 lexicon (or falls back to the fixture
// lexicon), appends non-HSK Foundations compounds from f0-inventory.tsv, generates
// stories via the pipeline (fixture or --live LLM), builds per-word Foundations cards
// + images from curated decisions, fetches/embeds real Commons JPEG thumbnails when
// --live, and emits a DevKey-signed zhuwen-a2-v0.zpack.
func cmdAppPack(args []string) error {
	out := flagValue(args, "--out")
	if out == "" {
		return fmt.Errorf("app-pack: --out <pack> required")
	}
	lexPath := flagValue(args, "--lexicon")
	foundationsDecisions := flagValue(args, "--foundations-decisions")
	canonCovers := flagValue(args, "--canon-covers")
	live := hasFlag(args, "--live")
	imagesLive := hasFlag(args, "--images-live")
	// --live implies --images-live
	if live {
		imagesLive = true
	}

	// --- 1. Load lexicon ---
	lex, err := loadLexicon(lexPath)
	if err != nil {
		return fmt.Errorf("app-pack: %w", err)
	}
	fmt.Printf("lexicon: %s (%d words)\n", lex.Version(), lex.Len())

	// --- 2. Append Foundations compound entries ---
	lex, err = appendFoundationsCompounds(lex)
	if err != nil {
		return fmt.Errorf("app-pack: %w", err)
	}

	// --- 3. Canon registry (always needed for cover image IDs) ---
	reg, err := assets.Canon()
	if err != nil {
		return fmt.Errorf("app-pack: canon: %w", err)
	}

	// --- 3b. Story pipeline (always run; fixture when offline, LLM when --live) ---
	spec := pipeline.BuildHSKBand(lex, "A2", 2, 3, 2)

	var inner gen.Provider
	if live {
		cfg, ok := gen.LLMConfigFromEnv()
		if !ok {
			return fmt.Errorf("app-pack --live: no API key (set ZHUWEN_LLM_API_KEY)")
		}
		cfg.Temperature = 0.9
		inner = gen.NewLLMProvider(cfg, lex)
		fmt.Printf("app-pack: LIVE (model %s @ %s)\n", cfg.Model, cfg.BaseURL)
	} else {
		inner = gen.NewBandFixtureProvider(func(id int) (string, bool) {
			if w, ok := lex.LookupID(id); ok {
				return w.Simp, true
			}
			return "", false
		})
		fmt.Println("app-pack: deterministic band-fixture provider (offline)")
	}

	// Constrained candidate-rerank (CP-09a).
	oversample := 6
	if v := flagValue(args, "--oversample"); v != "" {
		fmt.Sscanf(v, "%d", &oversample)
	}
	provider := gen.NewConstrainedProvider(inner, gen.GateConstraint{
		Dict:     lex.DictEntries(),
		Band:     gate.Band{Known: spec.Known, Frontier: spec.Frontier, Grammar: spec.Grammar},
		Detector: grammar.MarkerDetector{},
		MaxID:    lex.MaxID(),
		Cfg:      gate.DefaultConfig(),
	}, oversample, 80000)

	fmt.Printf("generating %d canon stories...\n", len(reg.All()))
	res, err := pipeline.Run(pipeline.Config{
		Lexicon:  lex,
		Registry: reg,
		Band:     spec,
		Provider: provider,
		GateCfg:  gate.DefaultConfig(),
	})
	if err != nil {
		return fmt.Errorf("app-pack: pipeline: %w", err)
	}
	fmt.Printf("pipeline: %d accepted, %d rejected\n", len(res.Stories), len(res.Rejected))

	// --- 4. Images: Foundations + canon covers ---
	var allImages []pack.Image
	var foundationsCards []pack.FoundationsCard
	fc := &images.FetchClient{}

	// Build word ID map.
	wordIDMap := make(map[string]int)
	for _, w := range lex.Words() {
		wordIDMap[w.Simp] = w.ID
	}

	// Build canon ID map.
	canonIDMap := make(map[string]string)
	for _, e := range reg.All() {
		canonIDMap[e.TitleZH] = e.CanonID
	}

	// Foundations decisions → images + cards.
	if foundationsDecisions != "" {
		decisions, err := images.LoadDecisions(foundationsDecisions)
		if err != nil {
			return fmt.Errorf("app-pack: foundations decisions: %w", err)
		}
		fmt.Printf("foundations decisions: %d entries\n", len(decisions))

		// Fetch provenance if images-live.
		var fProv images.ProvenanceStore
		if imagesLive {
			titles := collectCommonsTitles(decisions)
			if len(titles) > 0 {
				fProv, err = fc.FetchProvenanceByTitles(titles)
				if err != nil {
					fmt.Printf("WARNING: foundations provenance: %v (using stubs)\n", err)
					fProv = nil
				} else {
					fmt.Printf("foundations provenance: %d fetched\n", len(fProv))
				}
			}
		}

		var fImages []pack.Image
		if fProv != nil && len(fProv) > 0 {
			fImages, err = images.FoundationsDecisionImages(decisions, fProv, wordIDMap)
			if err != nil {
				return fmt.Errorf("app-pack: foundations images: %w", err)
			}
		} else {
			fImages = images.FoundationsDecisionImagesOffline(decisions, wordIDMap)
		}
		allImages = append(allImages, fImages...)
		fmt.Printf("foundations images: %d resolved\n", len(fImages))

		// Build per-word Foundations cards.
		foundationsCards = images.BuildFoundationsCards(decisions, lex)
		fmt.Printf("foundations cards: %d built\n", len(foundationsCards))
	}

	// Canon cover images — load curated pack.Image records directly.
	if canonCovers != "" {
		cImages, err := loadCuratedCoverImages(canonCovers)
		if err != nil {
			return fmt.Errorf("app-pack: canon covers: %w", err)
		}
		allImages = append(allImages, cImages...)
		fmt.Printf("cover images: %d loaded\n", len(cImages))
	}

	// --- 5. Embed real thumbnails (--live) ---
	if imagesLive && len(allImages) > 0 {
		embImages, err := images.EmbedThumbnails(allImages, fc, 480)
		if err != nil {
			fmt.Printf("WARNING: embed thumbnails: %v (continuing with stubs)\n", err)
		} else {
			allImages = embImages
			realCount := 0
			for _, im := range allImages {
				if len(im.Data) > 1000 {
					realCount++
				}
			}
			fmt.Printf("thumbnails: %d/%d images have real JPEG data\n", realCount, len(allImages))
		}
	}

	// --- 6. Build and sign the pack ---
	// Assign cover_image_id to each story from the canon cover images.
	coverByCanon := map[string]string{}
	for _, im := range allImages {
		if im.CanonID != "" {
			coverByCanon[im.CanonID] = im.ID
		}
	}
	for i := range res.Stories {
		if cid := coverByCanon[res.Stories[i].CanonID]; cid != "" {
			res.Stories[i].CoverImageID = cid
		} else {
			res.Stories[i].CoverImageID = "img-" + res.Stories[i].CanonID
		}
	}

	pub, priv := fixtures.DevKey()
	p := &pack.Pack{
		ID:               "zhuwen-a2",
		Semver:           "0.0.0",
		LexiconVersion:   lex.Version(),
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		Lexicon:          lex,
		Stories:          res.Stories,
		Images:           allImages,
		FoundationsCards: foundationsCards,
		Generator:        "zhuwenctl-app-pack",
		Model:            "deepseek-chat",
	}

	// Generate stub data for any images that are still nil.
	if err := patchStubImageData(p); err != nil {
		return fmt.Errorf("app-pack: stub images: %w", err)
	}

	if err := pack.Build(p, out, priv); err != nil {
		return fmt.Errorf("app-pack: build: %w", err)
	}
	pubPath := out + ".pub"
	if err := os.WriteFile(pubPath, []byte(pub.Encode()), 0o644); err != nil {
		return fmt.Errorf("app-pack: write pubkey: %w", err)
	}
	fmt.Printf("wrote %s (%d stories, %d images, %d foundations cards) + pubkey %s\n",
		out, len(p.Stories), len(allImages), len(foundationsCards), pubPath)
	return nil
}

// patchStubImageData generates unique stub PNG data for any image that still has nil Data
// (e.g. images whose thumbnails couldn't be fetched), so the pack builds without panicking
// and the on-device decoder always gets valid bytes.
func patchStubImageData(p *pack.Pack) error {
	encoded, err := images.EncodePackImages(p.Images, images.DefaultEncodeConfig())
	if err != nil {
		return err
	}
	p.Images = encoded
	return nil
}

// loadCuratedCoverImages loads canon cover images from a curated JSON file that is
// already in pack.Image format (ID, CanonID, File, W, H, License, ...). This is the
// output of the curate-canon pipeline, signed off and ready to embed.
func loadCuratedCoverImages(path string) ([]pack.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var imgs []pack.Image
	if err := json.Unmarshal(data, &imgs); err != nil {
		return nil, fmt.Errorf("parse curated cover images: %w", err)
	}
	// Override file extensions to .jpg (thumbnails are JPEG).
	for i := range imgs {
		imgs[i].Data = nil // clear data — thumbnails are fetched separately
		imgs[i].File = strings.Replace(imgs[i].File, "@480.heic", "@480.jpg", 1)
	}
	return imgs, nil
}
func loadLexicon(lexPath string) (*lexicon.Lexicon, error) {
	if lexPath != "" {
		return lexicon.ReadSQLite(lexPath)
	}
	return assets.Lexicon()
}

// collectCommonsTitles extracts Commons File: titles from a slice of image decisions.
func collectCommonsTitles(decisions []images.ImageDecision) []string {
	var titles []string
	for _, d := range decisions {
		if d.IsCommons() {
			titles = append(titles, d.CommonsTitle())
		}
	}
	return titles
}

// appendFoundationsCompounds reads f0-inventory.tsv and appends non-HSK compound
// forms (hsk3_level == 0) to the lexicon with stable synthetic IDs starting from 50000.
func appendFoundationsCompounds(lex *lexicon.Lexicon) (*lexicon.Lexicon, error) {
	candidates := []string{
		"factory/data/foundations/f0-inventory.tsv",
		"data/foundations/f0-inventory.tsv",
	}
	var tsvPath string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			tsvPath = p
			break
		}
	}
	if tsvPath == "" {
		return lex, nil
	}

	data, err := os.ReadFile(tsvPath)
	if err != nil {
		return lex, fmt.Errorf("reading f0-inventory.tsv: %w", err)
	}

	compoundBase := 50000
	nextID := compoundBase
	seen := map[string]bool{}
	for _, w := range lex.Words() {
		seen[w.Simp] = true
		if w.ID >= nextID {
			nextID = w.ID + 1
		}
	}

	var newWords []lexicon.Word
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 3 {
			continue
		}
		simp := strings.TrimSpace(cols[0])
		hsk := 0
		if len(cols) >= 5 {
			fmt.Sscanf(strings.TrimSpace(cols[4]), "%d", &hsk)
		}
		if hsk != 0 || simp == "" || seen[simp] {
			continue
		}
		pinyin := strings.TrimSpace(cols[1])
		en := ""
		if len(cols) >= 3 {
			en = strings.TrimSpace(cols[2])
		}
		newWords = append(newWords, lexicon.Word{
			ID:       nextID,
			Simp:     simp,
			Pinyin:   pinyin,
			HSK:      0,
			FreqRank: nextID,
			En:       en,
		})
		nextID++
	}
	if len(newWords) > 0 {
		allWords := append(append([]lexicon.Word{}, lex.Words()...), newWords...)
		sort.Slice(allWords, func(i, j int) bool { return allWords[i].ID < allWords[j].ID })
		lex, err = lexicon.FromWords(lex.Version(), allWords)
		if err != nil {
			return nil, err
		}
		fmt.Printf("compounds: appended %d non-HSK Foundations forms to lexicon\n", len(newWords))
	}
	return lex, nil
}
