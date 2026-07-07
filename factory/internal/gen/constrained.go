package gen

import (
	"fmt"
	"sort"

	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/segment"
)

// GateConstraint is the I1 gate configuration a ConstrainedProvider scores candidates
// against. It NEVER changes the gate budgets (I1): the provider only re-runs the exact same
// gate.Evaluate over an oversample and selects a passing candidate. Dict is the lexicon
// simp->id map; the per-story segmenter is rebuilt with each brief's proper-noun dictionary.
type GateConstraint struct {
	Dict     map[string]int
	Band     gate.Band
	Detector grammar.Detector
	MaxID    int
	Cfg      gate.Config
}

// tokenReporter is implemented by an inner provider that reports cumulative token usage
// (LLMProvider). It lets the ConstrainedProvider meter per-story spend for the token ceiling
// and the $-per-accepted-story metric. A fixture inner reports nothing (spend counts as 0).
type tokenReporter interface{ TokensUsed() int }

// ConstrainedProvider is the MC-2 fix for DeepSeek, which exposes no logit_bias/grammar
// constraint: candidate-rerank. It wraps an inner Provider (LLM or fixture), oversamples up
// to N candidates per story, segments + runs the SAME gate.Evaluate on each, and keeps the
// gate-passing one. A hard per-story token-budget ceiling aborts a story that blows it (the
// ceiling spans the whole repair loop for a story, keyed on brief.CanonID). It implements both
// Provider and RepairProvider so the repair loop can drive name-and-replace through it.
type ConstrainedProvider struct {
	inner     Provider
	con       GateConstraint
	n         int // oversample count per generation call (>=1)
	maxTokens int // per-story token ceiling across the whole repair loop (0 == unlimited)

	// cumulative telemetry (across all stories)
	totalCandidates int

	// per-story telemetry (reset when a new brief.CanonID arrives)
	curStory       string
	curStoryCand   int
	curStoryTokens int
	curStoryPassed bool
	curStoryAbort  bool
}

// NewConstrainedProvider wraps inner with candidate-rerank against con. n is the oversample
// count per generation call (clamped to >=1); maxTokens is the per-story token ceiling across
// the whole repair loop (0 disables the ceiling).
func NewConstrainedProvider(inner Provider, con GateConstraint, n, maxTokens int) *ConstrainedProvider {
	if n < 1 {
		n = 1
	}
	if con.Detector == nil {
		con.Detector = grammar.MarkerDetector{}
	}
	return &ConstrainedProvider{inner: inner, con: con, n: n, maxTokens: maxTokens}
}

// Retell oversamples the inner provider's first-pass generation and returns the best candidate.
func (p *ConstrainedProvider) Retell(b brief.Brief) (Story, error) {
	p.beginStory(b.CanonID)
	return p.oversample(b, func() (Story, error) { return p.inner.Retell(b) })
}

// RetellRepair oversamples the inner provider's repair generation (name-and-replace prompt),
// continuing the same per-story token budget. Falls back to plain Retell if the inner provider
// cannot repair.
func (p *ConstrainedProvider) RetellRepair(b brief.Brief, prior, repairPrompt string) (Story, error) {
	p.beginStory(b.CanonID)
	rp, ok := p.inner.(RepairProvider)
	if !ok {
		return p.oversample(b, func() (Story, error) { return p.inner.Retell(b) })
	}
	return p.oversample(b, func() (Story, error) { return rp.RetellRepair(b, prior, repairPrompt) })
}

// beginStory resets the per-story budget/telemetry when a new brief (canon_id) begins. The
// repair loop calls Retell then RetellRepair for one brief, so the ceiling spans the loop.
func (p *ConstrainedProvider) beginStory(canonID string) {
	if canonID != p.curStory {
		p.curStory = canonID
		p.curStoryCand = 0
		p.curStoryTokens = 0
		p.curStoryPassed = false
		p.curStoryAbort = false
	}
}

// oversample generates up to n candidates via mkCandidate, gates each, and returns the first
// passing one (or the closest-to-passing on exhaustion). It enforces the per-story token
// ceiling: once cumulative story spend reaches maxTokens without a pass, it aborts (returns an
// error so the repair loop discards the story — never loosening the gate).
func (p *ConstrainedProvider) oversample(b brief.Brief, mkCandidate func() (Story, error)) (Story, error) {
	seg := segment.New(p.con.Dict, b.Propers)
	var best Story
	bestDist := int(^uint(0) >> 1)
	haveBest := false

	for i := 0; i < p.n; i++ {
		if p.maxTokens > 0 && p.curStoryTokens >= p.maxTokens {
			p.curStoryAbort = true
			break
		}
		start := p.readTokens()
		s, err := mkCandidate()
		spent := p.readTokens() - start
		p.curStoryTokens += spent
		if err != nil {
			if haveBest {
				break
			}
			return Story{}, err
		}
		p.curStoryCand++
		p.totalCandidates++

		res := gate.Evaluate(seg.Segment(s.Text), p.con.Band, p.con.Detector, p.con.MaxID, p.con.Cfg)
		if res.Pass {
			p.curStoryPassed = true
			return s, nil
		}
		if d := candidateDistance(res); !haveBest || d < bestDist {
			best, bestDist, haveBest = s, d, true
		}
	}

	if !haveBest {
		return Story{}, fmt.Errorf("constrained: no candidate generated for %s", b.CanonID)
	}
	if p.maxTokens > 0 && p.curStoryTokens >= p.maxTokens {
		p.curStoryAbort = true
		return best, fmt.Errorf("constrained: per-story token ceiling %d exceeded for %s (spent %d, no gate pass)",
			p.maxTokens, b.CanonID, p.curStoryTokens)
	}
	// No pass this round, but budget remains: return the closest candidate so the repair loop
	// can name-and-replace and try again.
	return best, nil
}

// candidateDistance ranks losing candidates by how far they are from passing: over-budget
// (uncovered) tokens dominate, with heavy penalties for the hard-to-fix violations
// (out-of-frontier new types and out-of-lexicon literals). Lower is closer to passing.
func candidateDistance(res gate.Result) int {
	d := res.NewTokens
	for _, nt := range res.NewTypeCounts {
		if !nt.InFrontier {
			d += 50
		}
	}
	d += len(res.Literals) * 50
	return d
}

// readTokens returns the inner provider's cumulative token usage (0 for a fixture inner).
func (p *ConstrainedProvider) readTokens() int {
	if tr, ok := p.inner.(tokenReporter); ok {
		return tr.TokensUsed()
	}
	return 0
}

// TokensUsed returns the inner provider's cumulative token usage (spike cost metric).
func (p *ConstrainedProvider) TokensUsed() int { return p.readTokens() }

// TotalCandidates returns the cumulative number of candidates generated across all stories.
func (p *ConstrainedProvider) TotalCandidates() int { return p.totalCandidates }

// StoryCandidates returns the number of candidates generated for the current (most recent)
// story across all its repair iterations.
func (p *ConstrainedProvider) StoryCandidates() int { return p.curStoryCand }

// StoryTokens returns tokens spent on the current story across all its repair iterations.
func (p *ConstrainedProvider) StoryTokens() int { return p.curStoryTokens }

// StoryAborted reports whether the current story hit the per-story token ceiling without a pass.
func (p *ConstrainedProvider) StoryAborted() bool { return p.curStoryAbort }

// N returns the configured oversample count.
func (p *ConstrainedProvider) N() int { return p.n }

// MaxTokens returns the configured per-story token ceiling (0 == unlimited).
func (p *ConstrainedProvider) MaxTokens() int { return p.maxTokens }

// BandFixtureProvider is a deterministic, network-free constrained fixture: for any brief it
// synthesizes a guaranteed gate-passing story from words drawn from THAT brief's band — known
// filler words plus a single frontier word repeated MinRecurrence times. Unlike the fixed-list
// FixtureProvider it selects fillers from band.Known at retell time, so it passes on any
// non-degenerate band (fixture lexicon or the real hsk3.0-v1 A2/B1 bands) — the hermetic
// stand-in that reproduces gate-passing stories in `make ci` (I2: CI never hits the network).
type BandFixtureProvider struct {
	resolve   idSimp
	sentences int
	fillers   int
}

// grammarMarkers are the surface characters the detector keys on; the fixture must avoid using
// them as filler so it never trips a grammar pattern outside a band whitelist.
var grammarMarkers = map[string]bool{
	"把": true, "被": true, "了": true, "过": true, "的": true, "吗": true, "不": true, "在": true,
}

// idSimp maps a word ID to its simplified form.
type idSimp func(id int) (string, bool)

// NewBandFixtureProvider builds a band-aware deterministic fixture from an id->simp resolver.
func NewBandFixtureProvider(resolve idSimp) *BandFixtureProvider {
	return &BandFixtureProvider{resolve: resolve, sentences: 40, fillers: 5}
}

// Retell synthesizes a gate-passing story for the brief's band. Deterministic.
func (p *BandFixtureProvider) Retell(b brief.Brief) (Story, error) {
	if len(b.Frontier) == 0 {
		return Story{}, fmt.Errorf("band-fixture: brief %s has no frontier candidates", b.CanonID)
	}
	// Smallest frontier ID for determinism.
	fids := sortedIDs(b.Frontier)
	frontier, ok := p.resolve(fids[0])
	if !ok {
		return Story{}, fmt.Errorf("band-fixture: frontier id %d not resolvable", fids[0])
	}
	// Pick deterministic known filler simps (lowest IDs), skipping grammar markers, the
	// frontier word, and any declared proper noun.
	var fillers []string
	for _, id := range sortedIDs(b.Known) {
		s, ok := p.resolve(id)
		if !ok || grammarMarkers[s] || s == frontier {
			continue
		}
		if _, isProper := b.Propers[s]; isProper {
			continue
		}
		fillers = append(fillers, s)
		if len(fillers) >= p.fillers {
			break
		}
	}
	if len(fillers) < 3 {
		return Story{}, fmt.Errorf("band-fixture: brief %s band has <3 usable known fillers", b.CanonID)
	}
	return Story{
		CanonID: b.CanonID, TitleZH: b.TitleZH, TitleEN: b.TitleEN,
		Band: b.Band, Register: b.Register,
		Text:    emitStory(fillers, frontier, p.sentences),
		Fixture: true,
	}, nil
}

// emitStory writes `sentences` 5-word sentences, replacing the 3rd word of three evenly spaced
// sentences with the frontier word -> exactly 3 frontier occurrences (satisfies MinRecurrence).
func emitStory(fillers []string, frontier string, sentences int) string {
	const perSentence = 5
	at := map[int]bool{sentences / 4: true, sentences / 2: true, (3 * sentences) / 4: true}
	var b []rune
	fi := 0
	for s := 0; s < sentences; s++ {
		for w := 0; w < perSentence; w++ {
			if w == 2 && at[s] {
				b = append(b, []rune(frontier)...)
			} else {
				b = append(b, []rune(fillers[fi%len(fillers)])...)
				fi++
			}
		}
		b = append(b, '。')
	}
	return string(b)
}

func sortedIDs(set map[int]bool) []int {
	out := make([]int, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}
