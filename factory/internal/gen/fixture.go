package gen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/lexicon"
)

// FixtureProvider deterministically synthesizes a gate-passing story from known filler
// words plus a single frontier word repeated MinRecurrence+ times. The output is
// intentionally simple (flagged Fixture:true) — CP-01 exists to exercise the pipeline,
// not to produce shippable prose (that is the LLM's job at CP-09).
type FixtureProvider struct {
	lex         *lexicon.Lexicon
	fillerSimps []string // known, single-char, non-combining nouns
	sentences   int      // number of sentences to emit
}

// NewFixtureProvider builds a provider. fillerSimps must all be present in the lexicon
// and in the band's known set at retell time.
func NewFixtureProvider(lex *lexicon.Lexicon, fillerSimps []string) *FixtureProvider {
	return &FixtureProvider{lex: lex, fillerSimps: fillerSimps, sentences: 40}
}

// Retell produces a deterministic Story for the brief.
func (p *FixtureProvider) Retell(b brief.Brief) (Story, error) {
	if len(p.fillerSimps) < 3 {
		return Story{}, fmt.Errorf("fixture: need >=3 filler words, have %d", len(p.fillerSimps))
	}
	// Validate fillers are known.
	for _, s := range p.fillerSimps {
		w, ok := p.lex.LookupSimp(s)
		if !ok {
			return Story{}, fmt.Errorf("fixture: filler %q not in lexicon", s)
		}
		if !b.Known[w.ID] {
			return Story{}, fmt.Errorf("fixture: filler %q not in band known set", s)
		}
	}
	// Pick the smallest frontier candidate ID for determinism.
	if len(b.Frontier) == 0 {
		return Story{}, fmt.Errorf("fixture: brief %s has no frontier candidates", b.CanonID)
	}
	fids := make([]int, 0, len(b.Frontier))
	for id := range b.Frontier {
		fids = append(fids, id)
	}
	sort.Ints(fids)
	fw, ok := p.lex.LookupID(fids[0])
	if !ok {
		return Story{}, fmt.Errorf("fixture: frontier id %d not in lexicon", fids[0])
	}
	frontier := fw.Simp

	// Emit `sentences` sentences of 5 words each; replace the 3rd word of three evenly
	// spaced sentences with the frontier word -> exactly 3 frontier occurrences.
	const perSentence = 5
	frontierAt := map[int]bool{p.sentences / 4: true, p.sentences / 2: true, (3 * p.sentences) / 4: true}
	var sb strings.Builder
	fi := 0
	for s := 0; s < p.sentences; s++ {
		for w := 0; w < perSentence; w++ {
			if w == 2 && frontierAt[s] {
				sb.WriteString(frontier)
			} else {
				sb.WriteString(p.fillerSimps[fi%len(p.fillerSimps)])
				fi++
			}
		}
		sb.WriteString("。")
	}
	return Story{
		CanonID:  b.CanonID,
		TitleZH:  b.TitleZH,
		TitleEN:  b.TitleEN,
		Band:     b.Band,
		Register: b.Register,
		Text:     sb.String(),
		Fixture:  true,
	}, nil
}
