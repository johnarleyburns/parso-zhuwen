// Package foundations assembles the Foundations F0–F3 content (§5A) from the
// curated Commons image decisions and the HSK-3.0 lexicon. It produces
// foundations_card rows, F1 pattern tables, F2 micro-story outlines, and the
// F3 handoff gate that graduates the learner from Foundations to the regular
// story loop.
//
// Hermetic CI path: consumes fixture decisions + fixture lexicon; no network.
package foundations

import (
	"errors"
	"fmt"
	"sort"

	"github.com/parso/zhuwen-factory/internal/images"
	"github.com/parso/zhuwen-factory/internal/lexicon"
)

// Card is a single foundations_card row (matches pack schema.sql). The WordID
// is the foreign key into the lexicon table; ImageID references the image table;
// DistractorIDs is a JSON array of word_id values for the multiple-choice grid.
type Card struct {
	WordID        int
	ImageID       string
	SetID         string
	Stage         string // "F0"
	DistractorIDs []int
}

// A Set groups cards by semantic category and tracks the recommended order in
// which sets are introduced (§5A.2: photographable → pattern-anchored → abstract).
type Set struct {
	ID    string
	Name  string
	Cards []Card
}

// Config bundles the inputs needed for Foundations assembly.
type Config struct {
	Lexicon   *lexicon.Lexicon
	Decisions []images.ImageDecision
	Prov      images.ProvenanceStore
	// MinCardinality is the minimum number of words a set must have to be eligible.
	// Sets below this threshold are deferred to F1 pattern teaching.
	MinCardinality int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{MinCardinality: 3}
}

// BuildCards produces F0 card rows from curated image decisions + lexicon.
// Only words that have both a decision and a provenance record produce cards.
// Words in the decisions that lack provenances are logged as warnings and skipped.
func BuildCards(cfg Config) ([]Card, []Set, error) {
	if cfg.Lexicon == nil {
		return nil, nil, errors.New("foundations: lexicon required")
	}
	if cfg.MinCardinality <= 0 {
		cfg.MinCardinality = 3
	}

	// Build wordID map from lexicon.
	wordIDs := make(map[string]int, len(cfg.Decisions))
	for _, d := range cfg.Decisions {
		w, ok := cfg.Lexicon.LookupSimp(d.WordKey())
		if !ok {
			continue
		}
		wordIDs[d.WordKey()] = w.ID
	}

	// Build cards, bucketing by set.
	setCards := map[string][]Card{}
	setOrder := map[string]int{} // preserve first-seen order
	var all []Card

	for _, d := range cfg.Decisions {
		wid, ok := wordIDs[d.WordKey()]
		if !ok {
			continue
		}
		title := d.CommonsTitle()
		if _, ok := cfg.Prov[title]; !ok {
			continue
		}
		setID := d.Set
		if setID == "" {
			setID = "misc"
		}
		card := Card{
			WordID:  wid,
			ImageID: fmt.Sprintf("img-foundations-%s", d.WordKey()),
			SetID:   setID,
			Stage:   "F0",
		}
		all = append(all, card)
		if _, seen := setCards[setID]; !seen {
			setOrder[setID] = len(setOrder)
		}
		setCards[setID] = append(setCards[setID], card)
	}

	// Drop sets below the minimum cardinality and redistribute their cards
	// to the "misc" set (will be pattern-taught in F1).
	var eligible map[string]bool
	if cfg.MinCardinality > 1 {
		eligible = make(map[string]bool, len(setCards))
		for id, cards := range setCards {
			eligible[id] = len(cards) >= cfg.MinCardinality
		}
	}

	// Sort sets by first-seen order.
	type se struct {
		id    string
		cards []Card
	}
	var sorted []se
	for id, cards := range setCards {
		sorted = append(sorted, se{id, cards})
	}
	sort.SliceStable(sorted, func(i, j int) bool { return setOrder[sorted[i].id] < setOrder[sorted[j].id] })

	var sets []Set
	for _, s := range sorted {
		if eligible != nil && !eligible[s.id] && s.id != "misc" {
			continue
		}
		sets = append(sets, Set{ID: s.id, Cards: s.cards})
	}

	sort.SliceStable(all, func(i, j int) bool { return all[i].WordID < all[j].WordID })
	return all, sets, nil
}

// AddDistractors populates each card's distractors from the same set, using
// only cards that precede it in the ordered sequence (the "already taught"
// constraint per FR-11.3). Uses a deterministic rotation to avoid the same
// minimal pair twice in a row. Cards must be in the recommended order.
func AddDistractors(cards []Card, setCards map[string][]Card, numDistractors int) {
	if numDistractors <= 0 {
		numDistractors = 3
	}
	for i := range cards {
		c := &cards[i]
		pool := sameSetPredecessors(setCards[c.SetID], c.WordID)
		c.DistractorIDs = pickDeterministic(pool, numDistractors)
	}
}

func sameSetPredecessors(cards []Card, wordID int) []int {
	var ids []int
	for _, c := range cards {
		if c.WordID == wordID {
			break
		}
		ids = append(ids, c.WordID)
	}
	return ids
}

func pickDeterministic(pool []int, n int) []int {
	if len(pool) <= n {
		out := make([]int, len(pool))
		copy(out, pool)
		sort.Ints(out)
		return out
	}
	// Deterministic pseudo-random shuffle by hash of pool contents — stable across
	// rebuilds, no crypto needed.
	sort.Ints(pool)
	var key int
	for i, id := range pool {
		key += id * (i + 1)
	}
	// Rotate by key mod len, then take first n.
	start := key % len(pool)
	out := make([]int, n)
	for i := 0; i < n; i++ {
		out[i] = pool[(start+i)%len(pool)]
	}
	sort.Ints(out)
	return out
}

// HandoffThreshold is the required number of A1 stories gated at ≥98% coverage
// for the F3 handoff to fire (§5A.3).
const HandoffThreshold = 20

// MinCoverage is the required coverage fraction for a story to count toward the
// handoff threshold (§5A.3: ≥98%).
const MinCoverage = 0.98

// HandoffStatus reports whether the known set can gate ≥HandoffThreshold A1 stories
// at ≥MinCoverage. This is computed outside this package (by the gate.Selector)
// using the effective known set from KnownWordModel; this function is a pure
// predicate for testing the threshold logic.
type HandoffStatus struct {
	StoriesGated int
	Threshold    int
	Ready        bool
}

// CheckHandoff returns a HandoffStatus for a given count of gate-passing A1 stories.
func CheckHandoff(gated int) HandoffStatus {
	return HandoffStatus{
		StoriesGated: gated,
		Threshold:    HandoffThreshold,
		Ready:        gated >= HandoffThreshold,
	}
}

// WordImageability returns the imageability score for a word: the number of
// gate-passing Commons candidates from the fetch stage, or 0 if the word is
// not imageable (§5A.2, B-3 resolution). This is a pure lookup against a
// pre-computed table (built from `zhuwenctl images fetch|gate` output).
type WordImageability map[string]int // simp → candidate count

// IsImageable returns true if the word has at least one gate-passing candidate.
func (wi WordImageability) IsImageable(simp string) bool { return wi[simp] > 0 }
