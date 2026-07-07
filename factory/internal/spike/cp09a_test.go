package spike

import (
	"path/filepath"
	"testing"

	"github.com/parso/zhuwen-factory/internal/assets"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/gen"
	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/lexicon"
	"github.com/parso/zhuwen-factory/internal/pipeline"
	"github.com/parso/zhuwen-factory/internal/repair"
)

// TestConstrainedFixturePassesA2AndB1OnRealLexicon is the hermetic half of the CP-09a go/no-go
// gate: with NO network, the deterministic constrained band-fixture provider must reproduce
// gate-passing stories at A2 (known HSK<=2, frontier 3) AND B1 (known HSK<=4, frontier 5) on the
// real hsk3.0-v1 lexicon. This is what `make ci` verifies in place of the live --live run (I2:
// CI never hits the wire). The gate budgets (I1) are the locked DefaultConfig; nothing is
// loosened — the fixture simply satisfies them.
func TestConstrainedFixturePassesA2AndB1OnRealLexicon(t *testing.T) {
	lex, err := lexicon.IngestDir(filepath.Join("..", "..", "data", "hsk3.0"), "hsk3.0-v1")
	if err != nil {
		t.Fatalf("ingest real HSK lexicon: %v", err)
	}
	reg, err := assets.Canon()
	if err != nil {
		t.Fatal(err)
	}

	bands := []struct {
		name            string
		knownMax, front int
	}{
		{"A2", 2, 3},
		{"B1", 4, 5},
	}
	for _, bd := range bands {
		t.Run(bd.name, func(t *testing.T) {
			spec := pipeline.BuildHSKBand(lex, bd.name, bd.knownMax, bd.front, bd.knownMax)
			band := gate.Band{Known: spec.Known, Frontier: spec.Frontier, Grammar: spec.Grammar}
			checker := repair.PipelineChecker{
				Dict:     lex.DictEntries(),
				Band:     band,
				Detector: grammar.MarkerDetector{},
				MaxID:    lex.MaxID(),
				Cfg:      gate.DefaultConfig(),
			}
			inner := gen.NewBandFixtureProvider(func(id int) (string, bool) {
				if w, ok := lex.LookupID(id); ok {
					return w.Simp, true
				}
				return "", false
			})
			cp := gen.NewConstrainedProvider(inner, gen.GateConstraint{
				Dict:     lex.DictEntries(),
				Band:     band,
				Detector: grammar.MarkerDetector{},
				MaxID:    lex.MaxID(),
				Cfg:      gate.DefaultConfig(),
			}, 4, 0)

			sum := Run(lex, reg, spec, cp, checker, 5)
			if sum.Passed < 1 {
				t.Fatalf("%s: expected >=1 gate-passing story, got %d/%d (discarded %d)",
					bd.name, sum.Passed, sum.Entries, sum.Discarded)
			}
			// The deterministic fixture is engineered to pass on the first candidate.
			if sum.PassAtIter0 < 1 {
				t.Errorf("%s: expected >=1 pass@0, got %d", bd.name, sum.PassAtIter0)
			}
		})
	}
}
