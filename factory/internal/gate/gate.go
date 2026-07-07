// Package gate implements the coverage gate — product invariant I1 (00 §2, handoff §4.3).
//
// A story may only be recommended if its token coverage against the target band lexicon
// is >= 98% and every uncovered word type is a frontier candidate (or a glossed proper
// noun). This is enforced structurally: StoryCandidate has only unexported fields and is
// never returned in a valid (populated) state except by Evaluate. External packages
// cannot construct a passing candidate — they only ever get the zero value.
package gate

import (
	"fmt"
	"sort"

	"github.com/parso/zhuwen-factory/internal/bitset"
	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/segment"
)

// Config holds the pedagogy budgets (00 §6, PED-001..003).
type Config struct {
	MaxNewTypes      int     // <= 8 new types per story
	MaxNewTokenRatio float64 // <= 0.02 of running tokens
	MinRecurrence    int     // every new type occurs >= 3 times
	MinCoverage      float64 // >= 0.98 token coverage (derived cross-check)
}

// DefaultConfig returns the locked spec budgets.
func DefaultConfig() Config {
	return Config{MaxNewTypes: 8, MaxNewTokenRatio: 0.02, MinRecurrence: 3, MinCoverage: 0.98}
}

// Band is the target lexicon slice the gate evaluates against.
type Band struct {
	Known    map[int]bool    // known word IDs (band slice)
	Frontier map[int]bool    // frontier-candidate word IDs (allowed new types)
	Grammar  map[string]bool // whitelisted grammar-pattern IDs
}

// StoryCandidate is proof that a token stream passed the gate. I1: unexported fields,
// no exported constructor — only Evaluate produces a populated value.
type StoryCandidate struct {
	tokens         []segment.Token
	coverageBitmap []byte
	newTypeIDs     []int
	coverage       float64
	tokenCount     int
	typeCount      int
}

// Tokens returns the segmented token stream.
func (c *StoryCandidate) Tokens() []segment.Token { return c.tokens }

// CoverageBitmap returns the story's word-type bitmap (bit index == word ID).
func (c *StoryCandidate) CoverageBitmap() []byte { return c.coverageBitmap }

// NewTypeIDs returns the sorted new (frontier) word-type IDs introduced.
func (c *StoryCandidate) NewTypeIDs() []int { return c.newTypeIDs }

// Coverage returns the token coverage ratio in [0,1].
func (c *StoryCandidate) Coverage() float64 { return c.coverage }

// TokenCount returns the coverage-denominator token count (excludes proper nouns).
func (c *StoryCandidate) TokenCount() int { return c.tokenCount }

// TypeCount returns the number of distinct in-lexicon word types.
func (c *StoryCandidate) TypeCount() int { return c.typeCount }

// Reason codes are stable, machine-readable failure categories. They travel with the
// human-readable Reasons and are the contract the shared Go/Swift gate-vector suite
// asserts on (handoff §7). Adding a code here means adding it to the Swift gate too.
const (
	CodeLiteralOutOfLexicon = "literal_out_of_lexicon"
	CodeTypeBudget          = "type_budget"
	CodeTokenBudget         = "token_budget"
	CodeRecurrence          = "recurrence"
	CodeFrontier            = "frontier"
	CodeGrammar             = "grammar"
	CodeProperNounGloss     = "proper_noun_gloss"
)

// NewTypeStat describes one out-of-known word type present in the text: its stable word ID,
// how many times it occurred, and whether it is an allowed frontier candidate. These are
// OUTPUT-ONLY diagnostics consumed by the token-level repair loop (§4.4); they do not affect
// the pass/fail decision or any I1 budget.
type NewTypeStat struct {
	ID         int
	Count      int
	InFrontier bool
}

// Result is the outcome of a gate evaluation.
type Result struct {
	Pass      bool
	Reasons   []string
	Codes     []string        // sorted-unique machine reason codes (empty iff Pass)
	Candidate *StoryCandidate // non-nil iff Pass
	Coverage  float64
	// DenomTokens is the coverage denominator (running tokens, excluding proper nouns).
	// NewTokens is the numerator counted as "new" (new-type occurrences + literals).
	// Both are always populated; coverage == 1 - NewTokens/DenomTokens. They give the
	// vector suite language-neutral integers to compare (avoids float drift).
	DenomTokens int
	NewTokens   int
	// NewTypeCounts and Literals are output-only diagnostics for the repair loop: every
	// out-of-known type present (with occurrence count + frontier membership) and every
	// out-of-lexicon literal text. Always populated; never gate the decision (I1 unchanged).
	NewTypeCounts []NewTypeStat
	Literals      []string
}

// Evaluate runs the I1 gate over a segmented token stream (handoff §4.3).
func Evaluate(tokens []segment.Token, band Band, det grammar.Detector, maxWordID int, cfg Config) Result {
	var reasons []string
	codeSet := map[string]bool{}
	fail := func(code, reason string) {
		codeSet[code] = true
		reasons = append(reasons, reason)
	}

	// Denominator = running tokens excluding proper nouns (00 §6 proper-noun rule).
	denom := 0
	occ := map[int]int{}       // word id -> occurrences (Word kind only)
	typesSet := map[int]bool{} // distinct in-lexicon word types
	var literals []string
	for _, t := range tokens {
		switch t.Kind {
		case segment.Word:
			denom++
			occ[t.WordID]++
			typesSet[t.WordID] = true
		case segment.Literal:
			denom++
			literals = append(literals, t.Text)
		case segment.ProperNoun:
			// excluded from denominator
		}
	}

	// Out-of-lexicon literals cannot be covered.
	if len(literals) > 0 {
		fail(CodeLiteralOutOfLexicon, fmt.Sprintf("out-of-lexicon literal tokens (%d): %v", len(literals), literals))
	}

	// New types = in-lexicon types not in the known band slice.
	var newTypes []int
	for id := range typesSet {
		if !band.Known[id] {
			newTypes = append(newTypes, id)
		}
	}
	sort.Ints(newTypes)

	// Type budget.
	if len(newTypes) > cfg.MaxNewTypes {
		fail(CodeTypeBudget, fmt.Sprintf("new type budget exceeded: %d > %d", len(newTypes), cfg.MaxNewTypes))
	}

	// Token budget + coverage.
	newTokenCount := 0
	for _, id := range newTypes {
		newTokenCount += occ[id]
	}
	newTokenCount += len(literals)
	var ratio, coverage float64
	if denom > 0 {
		ratio = float64(newTokenCount) / float64(denom)
		coverage = 1 - ratio
	}
	if ratio > cfg.MaxNewTokenRatio {
		fail(CodeTokenBudget, fmt.Sprintf("new-token ratio %.4f > %.4f (coverage %.2f%%)", ratio, cfg.MaxNewTokenRatio, coverage*100))
	}

	// Recurrence: every new type occurs >= MinRecurrence.
	for _, id := range newTypes {
		if occ[id] < cfg.MinRecurrence {
			fail(CodeRecurrence, fmt.Sprintf("new type %d occurs %d time(s), needs >= %d", id, occ[id], cfg.MinRecurrence))
		}
	}

	// Output-only diagnostics for token-level repair (do not affect the decision).
	newTypeCounts := make([]NewTypeStat, 0, len(newTypes))
	for _, id := range newTypes {
		newTypeCounts = append(newTypeCounts, NewTypeStat{ID: id, Count: occ[id], InFrontier: band.Frontier[id]})
	}

	// Frontier discipline: every new type must be a frontier candidate.
	for _, id := range newTypes {
		if !band.Frontier[id] {
			fail(CodeFrontier, fmt.Sprintf("new type %d not in frontier queue", id))
		}
	}

	// Grammar gate: detected patterns must be a subset of the band whitelist.
	for _, p := range det.Detect(tokens) {
		if !band.Grammar[p] {
			fail(CodeGrammar, fmt.Sprintf("grammar pattern %q not in band whitelist", p))
		}
	}

	// Proper nouns: first occurrence requires a gloss.
	for _, t := range tokens {
		if t.Kind == segment.ProperNoun && t.First && t.Gloss == "" {
			fail(CodeProperNounGloss, fmt.Sprintf("proper noun %q lacks first-occurrence gloss", t.Text))
		}
	}

	if len(reasons) > 0 {
		return Result{Pass: false, Reasons: reasons, Codes: sortedCodes(codeSet), Coverage: coverage, DenomTokens: denom, NewTokens: newTokenCount, NewTypeCounts: newTypeCounts, Literals: literals}
	}

	// Build the coverage bitmap over all in-lexicon word types present.
	bm := bitset.New(maxWordID + 1)
	for id := range typesSet {
		bm.Set(id)
	}
	cand := &StoryCandidate{
		tokens:         tokens,
		coverageBitmap: bm.Bytes(),
		newTypeIDs:     newTypes,
		coverage:       coverage,
		tokenCount:     denom,
		typeCount:      len(typesSet),
	}
	return Result{Pass: true, Candidate: cand, Coverage: coverage, DenomTokens: denom, NewTokens: newTokenCount, NewTypeCounts: newTypeCounts, Literals: literals}
}

// sortedCodes returns the deduplicated failure codes in a stable order.
func sortedCodes(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
