// Package authored loads operator-written stories (A1–A2 backbone, CP-09b Part B) and
// runs them through the identical segment → gate.Evaluate path that generated stories
// use. No gate fork: authored and generated stories are indistinguishable to the
// verifier except by an origin/generator tag. An authored story that fails I1 is
// rejected with the same token-level diagnostics the repair loop uses, so the human
// author gets "name-and-replace"-style feedback in a fast local loop.
package authored

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/pack"
	"github.com/parso/zhuwen-factory/internal/segment"
)

// GeneratorTag is the manifest generator field for hand-authored stories.
const GeneratorTag = "hand-authored"

// ModelTag is the manifest model field (not applicable — no LLM).
const ModelTag = "none"

// Story is one operator-written story, ready for gate evaluation.
type Story struct {
	CanonID  string `json:"canon_id"`
	TitleZH  string `json:"title_zh"`
	TitleEN  string `json:"title_en"`
	Band     string `json:"band"`
	Text     string `json:"text"`
	Register string `json:"register"`
}

// Set is a collection of authored stories, loaded from JSON/TSV.
type Set struct {
	Stories []Story `json:"stories"`
	Origin  string  `json:"origin"`
}

// LoadSet reads an authored story set from a JSON file.
func LoadSet(path string) (*Set, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("authored: read %s: %w", path, err)
	}
	var s Set
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("authored: parse %s: %w", path, err)
	}
	if len(s.Stories) == 0 {
		return nil, fmt.Errorf("authored: %s contains no stories", path)
	}
	return &s, nil
}

// CheckResult is the outcome of gating one authored story.
type CheckResult struct {
	CanonID  string
	Pass     bool
	Reasons  []string
	Codes    []string
	Coverage float64
	// Token diagnostics: the specific out-of-vocabulary literals, new type counts, and
	// per-type recurrence info — same diagnostics the repair loop gives an LLM.
	Literals     []string
	NewTypeStats []gate.NewTypeStat
}

// Checker gates authored stories against the same I1 path.
type Checker struct {
	Seg      *segment.Segmenter
	Band     gate.Band
	Detector grammar.Detector
	MaxID    int
	Cfg      gate.Config
	Dict     map[string]int
}

// CheckStory gates one authored story through the I1 gate. Returns the result with
// token-level diagnostics suitable for author feedback.
func (c *Checker) CheckStory(s Story, propers map[string]string) CheckResult {
	seg := c.Seg
	if c.Dict != nil && propers != nil {
		seg = segment.New(c.Dict, propers)
	}
	det := c.Detector
	if det == nil {
		det = grammar.MarkerDetector{}
	}
	tokens := seg.Segment(s.Text)
	res := gate.Evaluate(tokens, c.Band, det, c.MaxID, c.Cfg)
	return CheckResult{
		CanonID:      s.CanonID,
		Pass:         res.Pass,
		Reasons:      res.Reasons,
		Codes:        res.Codes,
		Coverage:     res.Coverage,
		Literals:     res.Literals,
		NewTypeStats: res.NewTypeCounts,
	}
}

// CheckSet gates every story in a set. Returns the results with a pass-rate summary.
func (c *Checker) CheckSet(set *Set, propersMap map[string]map[string]string) ([]CheckResult, int, int) {
	var results []CheckResult
	passed := 0
	for _, s := range set.Stories {
		var propers map[string]string
		if propersMap != nil {
			propers = propersMap[s.CanonID]
		}
		cr := c.CheckStory(s, propers)
		results = append(results, cr)
		if cr.Pass {
			passed++
		}
	}
	return results, passed, len(set.Stories)
}

// FormatDiagnostics returns a human-readable summary of gate failures suitable for an
// author in a local editing loop (same diagnostics the repair loop uses).
func FormatDiagnostics(cr CheckResult, lex lookupSimpFn) string {
	if cr.Pass {
		return "PASS"
	}
	var out string
	out += fmt.Sprintf("FAIL (coverage=%.2f%%):\n", cr.Coverage*100)
	for _, c := range sortedUnique(cr.Codes) {
		out += fmt.Sprintf("  [%s]\n", c)
	}
	if len(cr.Literals) > 0 {
		out += "  out-of-lexicon literals: "
		for _, l := range cr.Literals {
			out += fmt.Sprintf("「%s」", l)
		}
		out += "\n"
	}
	if len(cr.NewTypeStats) > 0 {
		out += "  new type breakdown:\n"
		for _, nt := range cr.NewTypeStats {
			simp := fmt.Sprintf("#%d", nt.ID)
			if lex != nil {
				if s, ok := lex(nt.ID); ok {
					simp = s
				}
			}
			out += fmt.Sprintf("    「%s」: %d occurrences, in-frontier=%v\n",
				simp, nt.Count, nt.InFrontier)
		}
	}
	return out
}

type lookupSimpFn func(id int) (string, bool)

func sortedUnique(codes []string) []string {
	seen := map[string]bool{}
	for _, c := range codes {
		seen[c] = true
	}
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// ToPackStory converts an authored story (that has passed I1) plus its gate candidate
// into a pack.Story record. The story carries Origin="authored" and the generator tag so
// the manifest can record it.
func ToPackStory(s Story, res CheckResult, bodyTokens []pack.BodyToken, newTypeIDs []int, coverageBitmap []byte, canonOrigin string) pack.Story {
	storyID := fmt.Sprintf("%s-%s", s.CanonID, s.Band)
	imageID := "img-" + s.CanonID
	return pack.Story{
		ID:             storyID,
		TitleZH:        s.TitleZH,
		TitleEN:        s.TitleEN,
		Band:           s.Band,
		TokenCount:     len(bodyTokens),
		TypeCount:      len(newTypeIDs),
		CoverageBitmap: coverageBitmap,
		NewTypeIDs:     newTypeIDs,
		Topics:         []string{},
		Body:           bodyTokens,
		CanonID:        s.CanonID,
		Origin:         "authored",
		CoverImageID:   imageID,
		Fixture:        false,
	}
}

// BodyTokens converts segment tokens to pack.BodyToken records.
func BodyTokens(tokens []segment.Token) []pack.BodyToken {
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

// DictFromBrief builds a propers map from a canon entry (same as brief.Compile does),
// used to thread proper nouns into the segmenter when checking authored stories.
func DictFromBrief(e brief.Brief) map[string]string {
	return e.Propers
}
