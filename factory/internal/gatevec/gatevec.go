// Package gatevec is the single source of truth for the shared coverage-gate vector
// suite (handoff §7: "one shared invariant suite runs the same gate vectors through the
// Go and Swift implementations to prevent drift"). It defines deterministic gate inputs,
// runs each through the reference gate (internal/gate), and serializes inputs + expected
// outputs to JSON. genfixtures vendors that JSON to ios/Fixtures/gate-vectors.json, where
// the Swift CoverageGate must reproduce every field.
package gatevec

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/segment"
)

// FileName is the vendored artifact name.
const FileName = "gate-vectors.json"

// Tok is a language-neutral token in a vector.
type Tok struct {
	K     string `json:"k"` // "word" | "literal" | "proper"
	ID    int    `json:"id"`
	Text  string `json:"text"`
	Gloss string `json:"gloss,omitempty"`
	First bool   `json:"first,omitempty"`
}

// Expect is the reference gate's output for a vector (all fields language-neutral).
type Expect struct {
	Pass        bool     `json:"pass"`
	DenomTokens int      `json:"denomTokens"`
	NewTokens   int      `json:"newTokens"`
	CoverageBps int      `json:"coverageBps"` // round(coverage*10000), integer math
	NewTypeIDs  []int    `json:"newTypeIDs"`
	Codes       []string `json:"codes"`
}

// Vector is one gate input plus its reference expectation.
type Vector struct {
	Name      string   `json:"name"`
	Tokens    []Tok    `json:"tokens"`
	Known     []int    `json:"known"`
	Frontier  []int    `json:"frontier"`
	Grammar   []string `json:"grammar"`
	MaxWordID int      `json:"maxWordID"`
	Expect    Expect   `json:"expect"`
}

// ConfigJSON mirrors gate.Config for the manifest header.
type ConfigJSON struct {
	MaxNewTypes      int     `json:"maxNewTypes"`
	MaxNewTokenRatio float64 `json:"maxNewTokenRatio"`
	MinRecurrence    int     `json:"minRecurrence"`
	MinCoverage      float64 `json:"minCoverage"`
}

// Suite is the whole vendored artifact.
type Suite struct {
	GeneratedBy string     `json:"generatedBy"`
	Config      ConfigJSON `json:"config"`
	Vectors     []Vector   `json:"vectors"`
}

// input is a vector before its expectation is filled in.
type input struct {
	name      string
	tokens    []Tok
	known     []int
	frontier  []int
	grammar   []string
	maxWordID int
}

// word/literal/proper build tokens.
func word(id int, text string) Tok { return Tok{K: "word", ID: id, Text: text} }
func literal(text string) Tok      { return Tok{K: "literal", ID: -1, Text: text} }
func proper(text, gloss string, first bool) Tok {
	return Tok{K: "proper", ID: -1, Text: text, Gloss: gloss, First: first}
}

// rep returns n copies of a token.
func rep(t Tok, n int) []Tok {
	out := make([]Tok, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, t)
	}
	return out
}

func cat(groups ...[]Tok) []Tok {
	var out []Tok
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// inputs is the canonical, deterministic vector set. Word id 1 (山) is the known filler;
// ids 2/4 are frontier candidates; 把 (id 3) drives the grammar gate.
func inputs() []input {
	return []input{
		{
			name:   "pass_baseline",
			tokens: cat(rep(word(1, "山"), 197), rep(word(4, "坚持"), 3)),
			known:  []int{1}, frontier: []int{4}, maxWordID: 4,
		},
		{
			name:   "pass_full_known",
			tokens: rep(word(1, "山"), 200),
			known:  []int{1}, maxWordID: 1,
		},
		{
			name:   "pass_proper_noun_glossed",
			tokens: cat(rep(word(1, "山"), 200), []Tok{proper("后羿", "Hou Yi", true), proper("后羿", "", false)}),
			known:  []int{1}, maxWordID: 1,
		},
		{
			name:   "pass_grammar_whitelisted",
			tokens: cat(rep(word(1, "山"), 200), []Tok{word(3, "把")}),
			known:  []int{1, 3}, grammar: []string{"ba-construction"}, maxWordID: 3,
		},
		{
			name:   "fail_token_budget_979",
			tokens: cat(rep(word(1, "山"), 979), rep(word(2, "坚持"), 21)),
			known:  []int{1}, frontier: []int{2}, maxWordID: 2,
		},
		{
			name:   "fail_recurrence",
			tokens: cat(rep(word(1, "山"), 200), rep(word(2, "坚持"), 2)),
			known:  []int{1}, frontier: []int{2}, maxWordID: 2,
		},
		{
			name:   "fail_frontier_discipline",
			tokens: cat(rep(word(1, "山"), 200), rep(word(2, "坚持"), 3)),
			known:  []int{1}, maxWordID: 2,
		},
		{
			name:   "fail_grammar_not_whitelisted",
			tokens: cat(rep(word(1, "山"), 200), []Tok{word(3, "把")}),
			known:  []int{1, 3}, maxWordID: 3,
		},
		{
			name:   "fail_proper_noun_missing_gloss",
			tokens: cat(rep(word(1, "山"), 200), []Tok{proper("后羿", "", true)}),
			known:  []int{1}, maxWordID: 1,
		},
		{
			name:   "fail_literal_out_of_lexicon",
			tokens: cat(rep(word(1, "山"), 200), []Tok{literal("太")}),
			known:  []int{1}, maxWordID: 1,
		},
		{
			name:   "fail_type_budget",
			tokens: cat(rep(word(1, "山"), 200), buildManyNewTypes(2, 10)),
			known:  []int{1}, frontier: []int{2, 3, 4, 5, 6, 7, 8, 9, 10}, maxWordID: 10,
		},
		{
			name:   "fail_multi_literal_recurrence_frontier",
			tokens: cat(rep(word(1, "山"), 200), rep(word(2, "坚持"), 2), []Tok{literal("太")}),
			known:  []int{1}, maxWordID: 2,
		},
	}
}

// buildManyNewTypes returns ids [lo,hi] each repeated 3x (satisfies recurrence, blows the
// type budget when hi-lo+1 > MaxNewTypes).
func buildManyNewTypes(lo, hi int) []Tok {
	var out []Tok
	for id := lo; id <= hi; id++ {
		out = append(out, rep(word(id, "坚持"), 3)...)
	}
	return out
}

// toSegmentTokens converts vector tokens to gate input tokens.
func toSegmentTokens(toks []Tok) []segment.Token {
	out := make([]segment.Token, 0, len(toks))
	for _, t := range toks {
		switch t.K {
		case "word":
			out = append(out, segment.Token{Text: t.Text, WordID: t.ID, Kind: segment.Word})
		case "literal":
			out = append(out, segment.Token{Text: t.Text, WordID: -1, Kind: segment.Literal})
		case "proper":
			out = append(out, segment.Token{Text: t.Text, WordID: -1, Kind: segment.ProperNoun, Gloss: t.Gloss, First: t.First})
		}
	}
	return out
}

func bandFrom(known, frontier []int, gram []string) gate.Band {
	b := gate.Band{Known: map[int]bool{}, Frontier: map[int]bool{}, Grammar: map[string]bool{}}
	for _, k := range known {
		b.Known[k] = true
	}
	for _, f := range frontier {
		b.Frontier[f] = true
	}
	for _, g := range gram {
		b.Grammar[g] = true
	}
	return b
}

// coverageBps is the language-neutral coverage in basis points (round-half-up nearest).
func coverageBps(denom, newTok int) int {
	if denom <= 0 {
		return 0
	}
	covered := denom - newTok
	return (10000*covered + denom/2) / denom
}

// slice returns a non-nil empty slice so JSON encodes [] not null.
func slice(v []int) []int {
	if v == nil {
		return []int{}
	}
	return v
}
func sliceS(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// Build runs every input through the reference gate and returns fully-populated vectors.
func Build() []Vector {
	det := grammar.MarkerDetector{}
	cfg := gate.DefaultConfig()
	var out []Vector
	for _, in := range inputs() {
		toks := toSegmentTokens(in.tokens)
		r := gate.Evaluate(toks, bandFrom(in.known, in.frontier, in.grammar), det, in.maxWordID, cfg)
		var newTypeIDs []int
		if r.Candidate != nil {
			newTypeIDs = r.Candidate.NewTypeIDs()
		}
		out = append(out, Vector{
			Name:      in.name,
			Tokens:    in.tokens,
			Known:     slice(in.known),
			Frontier:  slice(in.frontier),
			Grammar:   sliceS(in.grammar),
			MaxWordID: in.maxWordID,
			Expect: Expect{
				Pass:        r.Pass,
				DenomTokens: r.DenomTokens,
				NewTokens:   r.NewTokens,
				CoverageBps: coverageBps(r.DenomTokens, r.NewTokens),
				NewTypeIDs:  slice(newTypeIDs),
				Codes:       sliceS(r.Codes),
			},
		})
	}
	return out
}

// suite assembles the full artifact.
func suite() Suite {
	cfg := gate.DefaultConfig()
	return Suite{
		GeneratedBy: "internal/gatevec via gate.Evaluate (reference I1 gate); do not edit by hand — run `make fixtures`",
		Config: ConfigJSON{
			MaxNewTypes:      cfg.MaxNewTypes,
			MaxNewTokenRatio: cfg.MaxNewTokenRatio,
			MinRecurrence:    cfg.MinRecurrence,
			MinCoverage:      cfg.MinCoverage,
		},
		Vectors: Build(),
	}
}

// JSON returns the deterministic, pretty-printed vendored artifact (no trailing newline).
func JSON() ([]byte, error) {
	return json.MarshalIndent(suite(), "", "  ")
}

// WriteJSON writes the vector suite to <dir>/gate-vectors.json.
func WriteJSON(dir string) error {
	b, err := JSON()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, FileName), b, 0o644)
}
