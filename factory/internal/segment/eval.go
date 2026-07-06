package segment

import "sort"

// EvalReport summarizes how well the frozen FMM segmenter covers a corpus and where it is
// ambiguous (handoff §4.3 / risk §13). It answers the MC-2.2 question — do we need jieba
// parity? — with data rather than vibes: high literal (out-of-dictionary) rates or many
// ambiguity hotspots argue for a stronger segmenter; low rates say FMM is fine for the band.
type EvalReport struct {
	Stories       int
	TotalTokens   int
	WordTokens    int
	ProperTokens  int
	LiteralTokens int
	// DistinctTypes is the number of distinct in-dictionary word IDs seen.
	DistinctTypes int
	// TokenCoverage = (Word+Proper)/Total; TypeCoverage = Word/(Word+Literal) token-wise.
	TokenCoverage float64
	// LiteralRate = LiteralTokens/Total — the fraction FMM could not resolve to the dictionary.
	LiteralRate float64
	// Hotspots are positions where a longer dictionary match overlaps a shorter one that FMM
	// committed to — i.e. places a different segmenter could disagree. Capped and sorted.
	Hotspots []Ambiguity
}

// Ambiguity is one place FMM's greedy choice could differ from another segmenter: at a given
// character offset, FMM took `Chosen` (length ChosenLen) but a dictionary word `Alternative`
// also started one character *inside* the chosen span (an overlap boundary).
type Ambiguity struct {
	StoryIdx int
	Rune     int    // character offset into the story
	Chosen   string // the token FMM emitted
	Overlap  string // a dictionary word overlapping the chosen token's interior
}

// Eval segments each story and computes coverage + ambiguity hotspots. `stories` are raw
// texts; the segmenter's dictionary/propers are whatever it was built with.
func (s *Segmenter) Eval(stories []string, maxHotspots int) EvalReport {
	var rep EvalReport
	rep.Stories = len(stories)
	typeSet := map[int]bool{}
	for si, text := range stories {
		toks := s.Segment(text)
		for _, t := range toks {
			rep.TotalTokens++
			switch t.Kind {
			case Word:
				rep.WordTokens++
				typeSet[t.WordID] = true
			case ProperNoun:
				rep.ProperTokens++
			case Literal:
				rep.LiteralTokens++
			}
		}
		rep.Hotspots = append(rep.Hotspots, s.hotspots(si, text)...)
	}
	rep.DistinctTypes = len(typeSet)
	if rep.TotalTokens > 0 {
		rep.TokenCoverage = float64(rep.WordTokens+rep.ProperTokens) / float64(rep.TotalTokens)
		rep.LiteralRate = float64(rep.LiteralTokens) / float64(rep.TotalTokens)
	}
	sort.Slice(rep.Hotspots, func(i, j int) bool {
		if rep.Hotspots[i].StoryIdx != rep.Hotspots[j].StoryIdx {
			return rep.Hotspots[i].StoryIdx < rep.Hotspots[j].StoryIdx
		}
		return rep.Hotspots[i].Rune < rep.Hotspots[j].Rune
	})
	if maxHotspots > 0 && len(rep.Hotspots) > maxHotspots {
		rep.Hotspots = rep.Hotspots[:maxHotspots]
	}
	return rep
}

// hotspots re-walks the text like Segment but flags positions where FMM's greedy longest
// match `chosen` overlaps another dictionary word starting inside it — the classic FMM/jieba
// disagreement site (e.g. 研究生命 → 研究生·命 vs 研究·生命).
func (s *Segmenter) hotspots(storyIdx int, text string) []Ambiguity {
	runes := []rune(text)
	var out []Ambiguity
	i := 0
	for i < len(runes) {
		r := runes[i]
		if terminators[r] || skippable(r) {
			i++
			continue
		}
		maxL := s.maxLen
		if maxL > len(runes)-i {
			maxL = len(runes) - i
		}
		chosenLen := 0
		var chosen string
		for l := maxL; l >= 1; l-- {
			cand := string(runes[i : i+l])
			if _, ok := s.propers[cand]; ok {
				chosenLen, chosen = l, cand
				break
			}
			if _, ok := s.dict[cand]; ok {
				chosenLen, chosen = l, cand
				break
			}
		}
		if chosenLen == 0 {
			i++ // literal
			continue
		}
		// Look for a dictionary word starting strictly inside the chosen span [i+1, i+chosenLen).
		if chosenLen >= 2 {
			for start := i + 1; start < i+chosenLen; start++ {
				remain := s.maxLen
				if remain > len(runes)-start {
					remain = len(runes) - start
				}
				found := ""
				for l := remain; l >= 2; l-- { // length-2+ overlaps are the interesting ones
					cand := string(runes[start : start+l])
					if _, ok := s.dict[cand]; ok {
						found = cand
						break
					}
				}
				if found != "" {
					out = append(out, Ambiguity{StoryIdx: storyIdx, Rune: i, Chosen: chosen, Overlap: found})
					break
				}
			}
		}
		i += chosenLen
	}
	return out
}
