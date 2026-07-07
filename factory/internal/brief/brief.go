// Package brief compiles a canon entry into a constrained retelling brief
// (handoff §4.2): a beat sheet plus the target lexicon slice (allowed word IDs =
// band lexicon; frontier candidates marked), length band, and register.
//
// CP-09b introduces the coverage-contract brief: a budget-aware brief that selects
// a small, deliberate set of ≤planNewTypes frontier words for the LLM to target,
// replacing the previous "dump the whole frontier" approach that both bled tokens
// and invited overuse (09a weakness: ~1/3 accept rate).
package brief

import (
	"sort"

	"github.com/parso/zhuwen-factory/internal/canon"
)

// BandSpec is the lexical envelope for a band (e.g. A2): the known word-ID slice,
// the frontier-candidate word IDs the gate will allow as new types, and the grammar
// whitelist for the band.
type BandSpec struct {
	Band      string
	Known     map[int]bool
	Frontier  map[int]bool
	Grammar   map[string]bool
	LengthMin int
	LengthMax int
	Register  string
	HSK3Level int
}

// Brief is the compiled instruction handed to the generation stage.
type Brief struct {
	CanonID    string
	TitleZH    string
	TitleEN    string
	Band       string
	Register   string
	HSK3Level  int
	Beats      []string
	Characters []canon.Character
	Known      map[int]bool
	Frontier   map[int]bool
	Grammar    map[string]bool
	LengthMin  int
	LengthMax  int
	// Propers is the per-story proper-noun dictionary (name -> gloss) threaded into the
	// segmenter so declared character names segment as ProperNoun and are excluded from the
	// coverage denominator (00 §6 proper-noun rule). This closes the MC-2 harness gap where
	// names became literal_out_of_lexicon. The gloss satisfies the first-occurrence gloss gate.
	Propers map[string]string
	// CP-09b coverage-contract fields. PlanNewTypes is a small, deliberately chosen subset
	// of the band frontier — ≤ MaxNewTypes (8) word IDs the LLM is instructed to target per
	// story. The budget-aware prompt lists only these, not the full frontier, focusing the
	// LLM on a concrete, achievable set and improving the accept rate (09a weakness).
	// PlanNewTypes ≤ the gate's MaxNewTypes — never a loosening of I1.
	PlanNewTypes []int
	// MinRecurrence is the explicit repetition contract (default 3, gate's MinRecurrence).
	MinRecurrence int
}

// Compile builds a Brief from a canon entry and a band spec.
func Compile(e canon.Entry, spec BandSpec) Brief {
	propers := map[string]string{}
	for _, c := range e.Characters {
		if c.NameZH == "" {
			continue
		}
		gloss := c.Gloss
		if gloss == "" {
			gloss = c.NameZH // gate requires a non-empty first-occurrence gloss
		}
		propers[c.NameZH] = gloss
	}
	return Brief{
		CanonID:    e.CanonID,
		TitleZH:    e.TitleZH,
		TitleEN:    e.TitleEN,
		Band:       spec.Band,
		Register:   spec.Register,
		HSK3Level:  spec.HSK3Level,
		Beats:      e.Beats,
		Characters: e.Characters,
		Known:      spec.Known,
		Frontier:   spec.Frontier,
		Grammar:    spec.Grammar,
		LengthMin:  spec.LengthMin,
		LengthMax:  spec.LengthMax,
		Propers:    propers,
		// Default: no deliberate frontier plan (uses full frontier set).
		PlanNewTypes:  nil,
		MinRecurrence: 3,
	}
}

// CompileWithPlan builds a Brief with an explicit coverage-contract frontier plan.
// planNewTypes must be a subset of spec.Frontier and ≤ MaxNewTypes (caller's
// responsibility; the gate will enforce at evaluation time).
func CompileWithPlan(e canon.Entry, spec BandSpec, planNewTypes []int, minRecurrence int) Brief {
	b := Compile(e, spec)
	b.PlanNewTypes = planNewTypes
	if minRecurrence > 0 {
		b.MinRecurrence = minRecurrence
	}
	return b
}

// PickFrontierWords selects a curated subset of up to k frontier word IDs for a
// coverage-contract brief. It picks the lowest-ID frontier words (deterministic, stable
// across builds), excluding proper-noun glosses when a lexicon lookup is available.
// k should be ≤ gate.MaxNewTypes (8) — this is an explicit coverage contract, not a loosening.
func PickFrontierWords(frontier map[int]bool, k int) []int {
	if k <= 0 || len(frontier) == 0 {
		return nil
	}
	ids := make([]int, 0, len(frontier))
	for id := range frontier {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if k > len(ids) {
		k = len(ids)
	}
	return ids[:k]
}

// FrontierSimps maps a set of word IDs to their simplified forms via a lookup function.
// Returns sorted by ID (deterministic). The lookup should return the simplified form and
// whether the ID is valid (typically from lexicon.LookupID).
func FrontierSimps(ids map[int]bool, lookup func(int) (string, bool)) []string {
	list := make([]int, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	sort.Ints(list)
	var out []string
	for _, id := range list {
		if simp, ok := lookup(id); ok {
			out = append(out, simp)
		}
	}
	return out
}

// SimpsForIDs maps a sorted list of word IDs to simplified forms via lookup.
func SimpsForIDs(ids []int, lookup func(int) (string, bool)) []string {
	var out []string
	for _, id := range ids {
		if simp, ok := lookup(id); ok {
			out = append(out, simp)
		}
	}
	return out
}
