package images

import (
	"fmt"
	"sort"

	"github.com/parso/zhuwen-factory/internal/lexicon"
	"github.com/parso/zhuwen-factory/internal/pack"
)

// BuildFoundationsCards reads curated image decisions and a lexicon to produce real
// per-word pack.FoundationsCard entries grouped by set. Each card gets its own per-word
// ImageID (img-foundations-<simp>). Distractors are drawn from same-set predecessors
// (words that appear earlier in the decision list within the same set). Words absent
// from the lexicon are silently filtered — never an error ("filter, don't fail").
func BuildFoundationsCards(decisions []ImageDecision, lex *lexicon.Lexicon) []pack.FoundationsCard {
	type entry struct {
		wordID int
		simp   string
		setID  string
	}
	// Resolve decisions against the lexicon, keeping only those whose simp resolves.
	var resolved []entry
	for _, d := range decisions {
		w, ok := lex.LookupSimp(d.key())
		if !ok {
			continue
		}
		resolved = append(resolved, entry{wordID: w.ID, simp: d.key(), setID: d.Set})
	}

	// Track same-set predecessors for distractors.
	bySet := map[string][]int{}
	var cards []pack.FoundationsCard
	for _, e := range resolved {
		distractors := append([]int(nil), bySet[e.setID]...)
		if len(distractors) > 3 {
			distractors = distractors[len(distractors)-3:]
		}
		sort.Ints(distractors)
		cards = append(cards, pack.FoundationsCard{
			WordID:        e.wordID,
			ImageID:       fmt.Sprintf("img-foundations-%s", e.simp),
			SetID:         e.setID,
			Stage:         "F0",
			DistractorIDs: distractors,
		})
		bySet[e.setID] = append(bySet[e.setID], e.wordID)
	}
	return cards
}

// FoundationsDecisionImages converts Foundations image decisions + provenance into
// per-word pack.Image records with ImageID = img-foundations-<simp>. Filters out
// decisions whose simp doesn't resolve in the lexicon (same "filter, don't fail"
// policy). Uses the same wordIDMap key format as DecisionsToImages.
func FoundationsDecisionImages(decisions []ImageDecision, prov ProvenanceStore, wordIDMap map[string]int) ([]pack.Image, error) {
	// Filter to decisions that resolve in the lexicon.
	var filtered []ImageDecision
	for _, d := range decisions {
		if _, ok := wordIDMap[d.key()]; ok {
			filtered = append(filtered, d)
		} else {
			// silently filter — this word isn't in the lexicon, skip it
		}
	}
	return DecisionsToImages(filtered, prov, wordIDMap)
}
