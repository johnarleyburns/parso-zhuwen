package repair

import (
	"strings"
	"testing"

	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/gen"
)

func TestRewritePromptNamesEveryViolation(t *testing.T) {
	res := gate.Result{
		Pass:    false,
		Codes:   []string{gate.CodeFrontier, gate.CodeRecurrence},
		Reasons: []string{"new type 42 not in frontier queue", "new type 7 occurs 1 time(s), needs >= 3"},
	}
	b := brief.Brief{Beats: []string{"农夫种田", "禾苗长大"}}
	prompt := RewritePrompt(res, b)

	for _, want := range []string{"至少 3 次", "允许新词列表", "new type 42 not in frontier queue", "农夫种田"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("repair prompt missing %q\n---\n%s", want, prompt)
		}
	}
}

func TestRewritePromptEmptyWhenNoFailures(t *testing.T) {
	if p := RewritePrompt(gate.Result{Pass: true}, brief.Brief{}); p != "" {
		t.Errorf("passing result should yield empty prompt, got %q", p)
	}
}

func TestHintsCoverAllGateCodes(t *testing.T) {
	all := []string{
		gate.CodeLiteralOutOfLexicon, gate.CodeTypeBudget, gate.CodeTokenBudget,
		gate.CodeRecurrence, gate.CodeFrontier, gate.CodeGrammar, gate.CodeProperNounGloss,
	}
	hints := HintsFromResult(gate.Result{Codes: all})
	if len(hints) != len(all) {
		t.Fatalf("got %d hints for %d codes — a gate code lacks a repair hint", len(hints), len(all))
	}
}

// --- loop tests ---

type scriptedProvider struct {
	texts []string
	calls int
}

func (s *scriptedProvider) Retell(b brief.Brief) (gen.Story, error) {
	txt := s.texts[len(s.texts)-1]
	if s.calls < len(s.texts) {
		txt = s.texts[s.calls]
	}
	s.calls++
	return gen.Story{Text: txt}, nil
}

// passOnChecker passes only when the text equals `good`.
type passOnChecker struct{ good string }

func (c passOnChecker) Check(text string) gate.Result {
	if text == c.good {
		return gate.Result{Pass: true}
	}
	return gate.Result{Pass: false, Reasons: []string{"fail"}, Codes: []string{gate.CodeRecurrence}}
}

func TestRepairLoopConverges(t *testing.T) {
	prov := &scriptedProvider{texts: []string{"bad", "bad", "good"}}
	fate := NewReprocessor(prov).Run(brief.Brief{CanonID: "c"}, passOnChecker{good: "good"})

	if !fate.Passed {
		t.Fatal("expected convergence")
	}
	if fate.Iterations != 2 {
		t.Errorf("iterations = %d, want 2", fate.Iterations)
	}
	if len(fate.Candidates) != 3 {
		t.Errorf("candidates = %d, want 3", len(fate.Candidates))
	}
}

func TestRepairLoopDiscardsAfterMax(t *testing.T) {
	prov := &scriptedProvider{texts: []string{"bad"}} // always bad
	fate := NewReprocessor(prov).Run(brief.Brief{}, passOnChecker{good: "good"})

	if fate.Passed {
		t.Fatal("expected discard")
	}
	// One initial attempt + MaxIterations repairs.
	if prov.calls != MaxIterations+1 {
		t.Errorf("provider calls = %d, want %d", prov.calls, MaxIterations+1)
	}
}

func TestRepairLoopFirstTryPass(t *testing.T) {
	prov := &scriptedProvider{texts: []string{"good"}}
	fate := NewReprocessor(prov).Run(brief.Brief{}, passOnChecker{good: "good"})
	if !fate.Passed || fate.Iterations != 0 {
		t.Errorf("expected first-try pass, got passed=%v iters=%d", fate.Passed, fate.Iterations)
	}
}
