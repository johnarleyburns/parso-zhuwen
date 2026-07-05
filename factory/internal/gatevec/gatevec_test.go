package gatevec

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/grammar"
)

// TestExpectationsAreSelfConsistent re-runs the reference gate over every vector and
// confirms the serialized expectation matches — Build() must faithfully record the gate's
// own output (this is the value the Swift side is held to).
func TestExpectationsAreSelfConsistent(t *testing.T) {
	det := grammar.MarkerDetector{}
	cfg := gate.DefaultConfig()
	for _, v := range Build() {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			r := gate.Evaluate(toSegmentTokens(v.Tokens), bandFrom(v.Known, v.Frontier, v.Grammar), det, v.MaxWordID, cfg)
			if r.Pass != v.Expect.Pass {
				t.Fatalf("pass = %v, expect %v (reasons %v)", r.Pass, v.Expect.Pass, r.Reasons)
			}
			if r.DenomTokens != v.Expect.DenomTokens || r.NewTokens != v.Expect.NewTokens {
				t.Errorf("denom/new = %d/%d, expect %d/%d", r.DenomTokens, r.NewTokens, v.Expect.DenomTokens, v.Expect.NewTokens)
			}
			if got := coverageBps(r.DenomTokens, r.NewTokens); got != v.Expect.CoverageBps {
				t.Errorf("coverageBps = %d, expect %d", got, v.Expect.CoverageBps)
			}
			if !equalStrings(v.Expect.Codes, sliceS(r.Codes)) {
				t.Errorf("codes = %v, expect %v", r.Codes, v.Expect.Codes)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestCoversEveryCodeAndAPass guarantees the suite exercises each machine reason code and
// at least one passing story, so the cross-impl check is meaningful.
func TestCoversEveryCodeAndAPass(t *testing.T) {
	want := map[string]bool{
		gate.CodeLiteralOutOfLexicon: false,
		gate.CodeTypeBudget:          false,
		gate.CodeTokenBudget:         false,
		gate.CodeRecurrence:          false,
		gate.CodeFrontier:            false,
		gate.CodeGrammar:             false,
		gate.CodeProperNounGloss:     false,
	}
	passes := 0
	for _, v := range Build() {
		if v.Expect.Pass {
			passes++
		}
		for _, c := range v.Expect.Codes {
			if _, ok := want[c]; !ok {
				t.Errorf("vector %s uses unknown code %q", v.Name, c)
			}
			want[c] = true
		}
	}
	if passes == 0 {
		t.Error("no passing vector in suite")
	}
	for code, seen := range want {
		if !seen {
			t.Errorf("no vector exercises code %q", code)
		}
	}
}

// TestVendoredVectorsUpToDate locks ios/Fixtures/gate-vectors.json to the current
// generation. If this fails, run `make fixtures` (or `cd factory && go run ./cmd/genfixtures`).
func TestVendoredVectorsUpToDate(t *testing.T) {
	want, err := JSON()
	if err != nil {
		t.Fatal(err)
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	// factory/internal/gatevec -> repo root -> ios/Fixtures/gate-vectors.json
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	path := filepath.Join(root, "ios", "Fixtures", FileName)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vendored vectors (%s): %v — run `make fixtures`", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("vendored %s is stale — run `make fixtures`", FileName)
	}
}
