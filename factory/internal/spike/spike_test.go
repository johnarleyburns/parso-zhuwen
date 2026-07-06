package spike

import (
	"testing"

	"github.com/parso/zhuwen-factory/internal/assets"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/gen"
	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/pipeline"
	"github.com/parso/zhuwen-factory/internal/repair"
	"github.com/parso/zhuwen-factory/internal/segment"
)

func TestFixtureHarnessAllPass(t *testing.T) {
	lex, err := assets.Lexicon()
	if err != nil {
		t.Fatal(err)
	}
	reg, err := assets.Canon()
	if err != nil {
		t.Fatal(err)
	}
	spec, err := pipeline.BuildFixtureBand(lex, assets.FrontierSimps())
	if err != nil {
		t.Fatal(err)
	}
	seg := segment.New(lex.DictEntries(), nil)
	checker := repair.PipelineChecker{
		Seg:      seg,
		Band:     gate.Band{Known: spec.Known, Frontier: spec.Frontier, Grammar: spec.Grammar},
		Detector: grammar.MarkerDetector{},
		MaxID:    lex.MaxID(),
		Cfg:      gate.DefaultConfig(),
	}
	provider := gen.NewFixtureProvider(lex, assets.FillerSimps())

	sum := Run(reg, spec, provider, checker, 5)
	if sum.Entries != 5 {
		t.Fatalf("entries = %d, want 5", sum.Entries)
	}
	// The fixture provider is built to pass the gate on the first try (mechanics validation).
	if sum.Passed != 5 || sum.PassAtIter0 != 5 || sum.Discarded != 0 {
		t.Errorf("passed=%d pass@0=%d discarded=%d, want 5/5/0", sum.Passed, sum.PassAtIter0, sum.Discarded)
	}
	if sum.PassRateAtIter0() != 1.0 || sum.MeanRepairIterations() != 0 {
		t.Errorf("rates wrong: pass@0=%f mean-iters=%f", sum.PassRateAtIter0(), sum.MeanRepairIterations())
	}
	if len(sum.Shipped) != 5 {
		t.Errorf("shipped = %d, want 5", len(sum.Shipped))
	}
}

func TestSummaryMathHelpers(t *testing.T) {
	s := Summary{Entries: 4, Passed: 3, Discarded: 1, SumRepairIters: 6, PassAtIter0: 1}
	if s.DiscardRate() != 0.25 {
		t.Errorf("discard rate = %f, want 0.25", s.DiscardRate())
	}
	if s.MeanRepairIterations() != 2.0 {
		t.Errorf("mean repair iters = %f, want 2.0", s.MeanRepairIterations())
	}
	if s.PassRateAtIter0() != 0.25 {
		t.Errorf("pass@0 rate = %f, want 0.25", s.PassRateAtIter0())
	}
}
