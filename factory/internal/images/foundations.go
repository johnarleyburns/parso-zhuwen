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
	var resolved []entry
	for _, d := range decisions {
		w, ok := lex.LookupSimp(d.key())
		if !ok {
			continue
		}
		resolved = append(resolved, entry{wordID: w.ID, simp: d.key(), setID: d.Set})
	}

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
// policy).
func FoundationsDecisionImages(decisions []ImageDecision, prov ProvenanceStore, wordIDMap map[string]int) ([]pack.Image, error) {
	var filtered []ImageDecision
	for _, d := range decisions {
		if _, ok := wordIDMap[d.key()]; ok {
			filtered = append(filtered, d)
		}
	}
	return DecisionsToImages(filtered, prov, wordIDMap)
}

// FoundationsDecisionImagesOffline creates per-word pack.Image records from
// decisions without requiring provenance (offline/CI path). Each image gets a
// placeholder W/H and a stub file path; Data is nil so EncodePackImages fills
// in valid 1x1 PNG stubs. Never fails — silently filters words absent from the
// lexicon.
func FoundationsDecisionImagesOffline(decisions []ImageDecision, wordIDMap map[string]int) []pack.Image {
	var imgs []pack.Image
	for _, d := range decisions {
		wid := wordIDMap[d.key()]
		if wid == 0 {
			continue
		}
		id := fmt.Sprintf("img-foundations-%s", d.key())
		imgs = append(imgs, pack.Image{
			ID:          id,
			WordID:      &wid,
			File:        fmt.Sprintf("images/%s@480.jpg", id),
			W:           480,
			H:           480,
			License:     "placeholder",
			LicenseURL:  "https://commons.wikimedia.org",
			Author:      "placeholder",
			SourceURL:   "https://commons.wikimedia.org",
			RetrievedAt: "placeholder",
		})
	}
	return imgs
}

// CanonDecisionImagesOffline creates pack.Image records for story covers from
// canon decisions without requiring provenance (offline/CI path). Each image
// gets a placeholder file path and nil Data; EncodePackImages fills in 1x1
// PNG stubs. Never fails.
func CanonDecisionImagesOffline(decisions []ImageDecision, canonIDMap map[string]string) []pack.Image {
	var imgs []pack.Image
	for _, d := range decisions {
		if d.Decision == "" || d.Decision == "__reject__" {
			continue
		}
		canonID, ok := canonIDMap[d.key()]
		if !ok || canonID == "" {
			continue
		}
		id := "img-" + canonID
		imgs = append(imgs, pack.Image{
			ID:          id,
			CanonID:     canonID,
			File:        fmt.Sprintf("images/%s@480.jpg", id),
			W:           480,
			H:           480,
			License:     "placeholder",
			LicenseURL:  "https://commons.wikimedia.org",
			Author:      "placeholder",
			SourceURL:   "https://commons.wikimedia.org",
			RetrievedAt: "placeholder",
		})
	}
	return imgs
}
