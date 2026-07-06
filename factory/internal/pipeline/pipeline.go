// Package pipeline wires the factory stages for CP-01: canon -> brief -> gen -> segment
// -> gate -> pack inputs (handoff §4). Every stage is a pure function of its input so
// the whole run is deterministic and testable offline.
package pipeline

import (
	"fmt"
	"sort"

	"github.com/parso/zhuwen-factory/internal/align"
	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/canon"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/gen"
	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/lexicon"
	"github.com/parso/zhuwen-factory/internal/pack"
	"github.com/parso/zhuwen-factory/internal/segment"
)

// Rejected records a brief that failed the gate, with reasons (repair-loop input, §4.4).
type Rejected struct {
	CanonID string
	Reasons []string
}

// Result is the outcome of a pipeline run.
type Result struct {
	Stories   []pack.Story
	Questions []pack.Question
	Images    []pack.Image
	Rejected  []Rejected
}

// Config bundles everything a run needs.
type Config struct {
	Lexicon  *lexicon.Lexicon
	Registry *canon.Registry
	Band     brief.BandSpec
	Provider gen.Provider
	GateCfg  gate.Config
	Detector grammar.Detector
	Propers  map[string]string // proper-noun glosses for the segmenter
}

// Run executes the pipeline over every canon entry.
func Run(cfg Config) (Result, error) {
	if cfg.Detector == nil {
		cfg.Detector = grammar.MarkerDetector{}
	}
	seg := segment.New(cfg.Lexicon.DictEntries(), cfg.Propers)
	band := gate.Band{Known: cfg.Band.Known, Frontier: cfg.Band.Frontier, Grammar: cfg.Band.Grammar}
	maxID := cfg.Lexicon.MaxID()

	var res Result
	imgSeen := map[string]bool{}
	for _, e := range cfg.Registry.All() {
		b := brief.Compile(e, cfg.Band)
		story, err := cfg.Provider.Retell(b)
		if err != nil {
			return Result{}, fmt.Errorf("retell %s: %w", e.CanonID, err)
		}
		tokens := seg.Segment(story.Text)
		gr := gate.Evaluate(tokens, band, cfg.Detector, maxID, cfg.GateCfg)
		if !gr.Pass {
			res.Rejected = append(res.Rejected, Rejected{CanonID: e.CanonID, Reasons: gr.Reasons})
			continue
		}
		cand := gr.Candidate

		storyID := fmt.Sprintf("%s-%s", e.CanonID, cfg.Band.Band)
		imageID := "img-" + e.CanonID

		// Forced-alignment stage (§4.7): word-level timings for karaoke (FR-5.1). Audio
		// bytes are rendered in pack.Build (stub at fixture tiers; CosyVoice at CP-09).
		alignRows, _ := align.Align(tokens, align.DefaultConfig())
		audioFile := fmt.Sprintf("audio/%s.opus", storyID)

		// Reuse rule §8A.1(a): all band-retellings of a canon entry share one image.
		if !imgSeen[imageID] {
			imgSeen[imageID] = true
			src := ""
			if len(e.SourceURLs) > 0 {
				src = e.SourceURLs[0]
			}
			res.Images = append(res.Images, pack.Image{
				ID:      imageID,
				CanonID: e.CanonID,
				File:    fmt.Sprintf("images/%s@480.heic", imageID),
				W:       480, H: 480,
				License:     "PD",
				LicenseURL:  "https://creativecommons.org/publicdomain/mark/1.0/",
				Author:      "Wikimedia Commons contributors",
				SourceURL:   src,
				RetrievedAt: "2026-07-04",
			})
		}

		res.Stories = append(res.Stories, pack.Story{
			ID:             storyID,
			TitleZH:        story.TitleZH,
			TitleEN:        story.TitleEN,
			Band:           cfg.Band.Band,
			HSK3Level:      cfg.Band.HSK3Level,
			TokenCount:     cand.TokenCount(),
			TypeCount:      cand.TypeCount(),
			CoverageBitmap: cand.CoverageBitmap(),
			NewTypeIDs:     cand.NewTypeIDs(),
			Topics:         []string{},
			GrammarIDs:     cfg.Detector.Detect(tokens),
			Body:           bodyFromTokens(tokens),
			CanonID:        e.CanonID,
			Tier:           e.Tier,
			Origin:         e.Origin,
			SourceURLs:     e.SourceURLs,
			PDRationale:    e.PDRationale,
			CoverImageID:   imageID,
			Fixture:        story.Fixture,
			AudioFile:      audioFile,
			Alignment:      alignRows,
		})
		res.Questions = append(res.Questions, stubQuestions(storyID, cfg.Band.Band)...)
	}
	return res, nil
}

func bodyFromTokens(tokens []segment.Token) []pack.BodyToken {
	out := make([]pack.BodyToken, 0, len(tokens))
	for _, t := range tokens {
		bt := pack.BodyToken{S: t.SentenceIdx}
		switch t.Kind {
		case segment.Word:
			bt.W = t.WordID
		case segment.ProperNoun:
			bt.W = -1
			bt.Literal = t.Text
			bt.PN = true
		default:
			bt.W = -1
			bt.Literal = t.Text
		}
		out = append(out, bt)
	}
	return out
}

// stubQuestions produces 3 deterministic MC questions (CP-01 stub; real generation §4.5).
func stubQuestions(storyID, band string) []pack.Question {
	var qs []pack.Question
	for i := 1; i <= 3; i++ {
		qs = append(qs, pack.Question{
			ID:        fmt.Sprintf("%s-q%d", storyID, i),
			StoryID:   storyID,
			PromptZH:  fmt.Sprintf("问题 %d", i),
			Options:   []string{"甲", "乙", "丙", "丁"},
			AnswerIdx: 0,
			Band:      band,
		})
	}
	return qs
}

// BuildFixtureBand derives an A2 fixture band from the lexicon: everything known except
// the named frontier words, which become the allowed new types.
func BuildFixtureBand(lex *lexicon.Lexicon, frontierSimps []string) (brief.BandSpec, error) {
	frontier := map[int]bool{}
	for _, s := range frontierSimps {
		w, ok := lex.LookupSimp(s)
		if !ok {
			return brief.BandSpec{}, fmt.Errorf("frontier word %q not in lexicon", s)
		}
		frontier[w.ID] = true
	}
	known := map[int]bool{}
	for _, w := range lex.Words() {
		if !frontier[w.ID] {
			known[w.ID] = true
		}
	}
	grammarWhitelist := map[string]bool{
		"le-aspect": true, "de-attributive": true, "bu-negation": true,
		"zai-progressive": true, "guo-aspect": true, "ma-question": true,
	}
	return brief.BandSpec{
		Band: "A2", Known: known, Frontier: frontier, Grammar: grammarWhitelist,
		LengthMin: 120, LengthMax: 400, Register: "narrative", HSK3Level: 2,
	}, nil
}

// BuildHSKBand derives a band from the real HSK-3.0 lexicon: the known set is every word at or
// below knownMaxLevel, the frontier-candidate set is every word at exactly frontierLevel, and the
// grammar whitelist is the A2 set. This is the real content-bet band for the MC-2 spike — e.g.
// A2 = known HSK 1–2, frontier HSK 3.
func BuildHSKBand(lex *lexicon.Lexicon, band string, knownMaxLevel, frontierLevel, hsk3Level int) brief.BandSpec {
	known := map[int]bool{}
	frontier := map[int]bool{}
	for _, w := range lex.Words() {
		switch {
		case w.HSK <= knownMaxLevel:
			known[w.ID] = true
		case w.HSK == frontierLevel:
			frontier[w.ID] = true
		}
	}
	grammarWhitelist := map[string]bool{
		// The standard, teachable grammar points the detector recognizes; whitelisting them is
		// band configuration (which structures A2+ learners have met), not an I1 budget change.
		"le-aspect": true, "de-attributive": true, "bu-negation": true,
		"zai-progressive": true, "guo-aspect": true, "ma-question": true,
		"ba-construction": true, "bei-construction": true,
	}
	return brief.BandSpec{
		Band: band, Known: known, Frontier: frontier, Grammar: grammarWhitelist,
		LengthMin: 120, LengthMax: 400, Register: "narrative", HSK3Level: hsk3Level,
	}
}

// SortedNewTypeSimps is a helper for demo output.
func SortedNewTypeSimps(lex *lexicon.Lexicon, ids []int) []string {
	cp := append([]int(nil), ids...)
	sort.Ints(cp)
	var out []string
	for _, id := range cp {
		if w, ok := lex.LookupID(id); ok {
			out = append(out, w.Simp)
		}
	}
	return out
}
