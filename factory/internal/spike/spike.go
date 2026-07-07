// Package spike runs the MC-2 content-reality harness: canon → brief → gen → segment → gate
// → repair over a set of entries, aggregating the metrics the spike report needs. It is
// provider-agnostic: with the deterministic fixture provider it validates the pipeline
// mechanics hermetically; with the LLM provider (behind ZHUWEN_LLM_API_KEY) it produces the
// real content-bet numbers. Gate budgets (I1) are never altered here.
package spike

import (
	"sort"

	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/canon"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/gen"
	"github.com/parso/zhuwen-factory/internal/lexicon"
	"github.com/parso/zhuwen-factory/internal/repair"
)

// Summary aggregates the metrics MC-2.5 / CP-09a require.
type Summary struct {
	Entries         int
	PassAtIter0     int            // briefs whose first candidate passed
	Passed          int            // briefs that eventually passed (within MaxIterations)
	Discarded       int            // briefs never passing
	SumRepairIters  int            // sum of iterations among passed briefs
	TotalTokens     int            // cumulative LLM tokens (0 for fixture provider)
	TotalCandidates int            // cumulative rerank candidates generated (mean N = /Entries)
	FailureCodeHist map[string]int // stable failure code → count across all failed attempts
	Shipped         []gen.Story    // passing stories (for verbatim samples)
	Fates           []repair.Fate
}

// MeanCandidates is the average number of rerank candidates generated per story (mean N).
func (s Summary) MeanCandidates() float64 {
	if s.Entries == 0 {
		return 0
	}
	return float64(s.TotalCandidates) / float64(s.Entries)
}

// TokensPerAccepted is the mean LLM tokens spent per accepted (passing) story — the core
// economics number for the go/no-go decision. Returns 0 when nothing passed.
func (s Summary) TokensPerAccepted() float64 {
	if s.Passed == 0 {
		return 0
	}
	return float64(s.TotalTokens) / float64(s.Passed)
}

// candidateReporter is implemented by the ConstrainedProvider so the spike can record how many
// rerank candidates each story consumed.
type candidateReporter interface{ StoryCandidates() int }

// MeanRepairIterations is the average number of repair iterations among briefs that passed
// (0 == passed first try). NaN-safe: returns 0 when nothing passed.
func (s Summary) MeanRepairIterations() float64 {
	if s.Passed == 0 {
		return 0
	}
	return float64(s.SumRepairIters) / float64(s.Passed)
}

// DiscardRate is discarded / entries.
func (s Summary) DiscardRate() float64 {
	if s.Entries == 0 {
		return 0
	}
	return float64(s.Discarded) / float64(s.Entries)
}

// PassRateAtIter0 is first-try passes / entries.
func (s Summary) PassRateAtIter0() float64 {
	if s.Entries == 0 {
		return 0
	}
	return float64(s.PassAtIter0) / float64(s.Entries)
}

// Run executes the harness over up to `limit` entries of the registry (0 == all), compiling a
// brief per entry, running it through the repair loop against a per-brief checker (threading
// the brief's proper-noun dictionary), and aggregating. `lex` enables token-level
// name-and-replace repair and per-brief segmentation; it may be nil for the fixture harness.
func Run(lex *lexicon.Lexicon, reg *canon.Registry, spec brief.BandSpec, provider gen.Provider, checker repair.PipelineChecker, limit int) Summary {
	rp := repair.NewReprocessor(provider)
	rp.Lex = lex
	rp.Band = gate.Band{Known: spec.Known, Frontier: spec.Frontier, Grammar: spec.Grammar}
	sum := Summary{FailureCodeHist: map[string]int{}}

	entries := reg.All()
	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}
	cr, _ := provider.(candidateReporter)
	for _, e := range entries {
		b := brief.Compile(e, spec)
		fate := rp.Run(b, checker.ForBrief(b.Propers))
		sum.Entries++
		sum.Fates = append(sum.Fates, fate)
		sum.TotalTokens += fate.TotalTokens
		if cr != nil {
			sum.TotalCandidates += cr.StoryCandidates()
		}

		for _, codes := range fate.FailCodes {
			for _, c := range codes {
				sum.FailureCodeHist[c]++
			}
		}
		if fate.Passed {
			sum.Passed++
			sum.SumRepairIters += fate.Iterations
			if fate.Iterations == 0 {
				sum.PassAtIter0++
			}
			if len(fate.Candidates) > 0 {
				sum.Shipped = append(sum.Shipped, fate.Candidates[len(fate.Candidates)-1])
			}
		} else {
			sum.Discarded++
		}
	}
	return sum
}

// SortedFailureCodes returns the histogram keys in descending count order (stable ties).
func (s Summary) SortedFailureCodes() []string {
	keys := make([]string, 0, len(s.FailureCodeHist))
	for k := range s.FailureCodeHist {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if s.FailureCodeHist[keys[i]] != s.FailureCodeHist[keys[j]] {
			return s.FailureCodeHist[keys[i]] > s.FailureCodeHist[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}
