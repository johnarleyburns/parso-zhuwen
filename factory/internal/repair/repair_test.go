package repair

import (
	"strings"
	"testing"

	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/gen"
	"github.com/parso/zhuwen-factory/internal/lexicon"
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

// --- token-level name-and-replace ---

func nameReplaceLex(t *testing.T) *lexicon.Lexicon {
	t.Helper()
	// 1山 2水 4人 = known; 3坚持 = frontier; 99徘徊 = out-of-frontier new type.
	lex, err := lexicon.Ingest(strings.NewReader(
		"1\t山\tshān\t1\t10\t\n2\t水\tshuǐ\t1\t11\t\n3\t坚持\tjiānchí\t3\t900\t\n4\t人\trén\t1\t5\t\n99\t徘徊\tpáihuái\t6\t9000\t\n"),
		"nr-v1")
	if err != nil {
		t.Fatal(err)
	}
	return lex
}

func TestNameReplacePromptNamesSpecificTokens(t *testing.T) {
	lex := nameReplaceLex(t)
	band := gate.Band{
		Known:    map[int]bool{1: true, 2: true, 4: true},
		Frontier: map[int]bool{3: true},
		Grammar:  map[string]bool{},
	}
	// Result naming: 徘徊 (99) used but out-of-frontier; 坚持 (3) under-recurring (1x); literal 太.
	res := gate.Result{
		Pass:  false,
		Codes: []string{gate.CodeFrontier, gate.CodeRecurrence, gate.CodeLiteralOutOfLexicon},
		NewTypeCounts: []gate.NewTypeStat{
			{ID: 99, Count: 4, InFrontier: false},
			{ID: 3, Count: 1, InFrontier: true},
		},
		Literals: []string{"太", "太"},
	}
	b := brief.Brief{Beats: []string{"农夫种田"}}
	prompt := NameReplacePrompt(res, b, lex, band)

	for _, want := range []string{"徘徊", "坚持", "太", "农夫种田"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("name-and-replace prompt missing %q\n---\n%s", want, prompt)
		}
	}
	// The out-of-frontier offender must be offered an in-band substitute (a known word).
	if !strings.Contains(prompt, "山") && !strings.Contains(prompt, "人") && !strings.Contains(prompt, "水") {
		t.Errorf("prompt did not offer any known-set substitute:\n%s", prompt)
	}
	// Deduped literal: 太 should appear as a single named offender line, not twice.
	if strings.Count(prompt, "「太」") != 1 {
		t.Errorf("literal not deduped; got %d occurrences", strings.Count(prompt, "「太」"))
	}
}

func TestNameReplaceEmptyOnPass(t *testing.T) {
	if p := NameReplacePrompt(gate.Result{Pass: true}, brief.Brief{}, nameReplaceLex(t), gate.Band{}); p != "" {
		t.Errorf("passing result should yield empty name-and-replace prompt, got %q", p)
	}
}

// convergingChecker fails until it sees a text containing "坚持坚持坚持" (>=3 recurrences), to
// simulate a real gate pulling toward the budget after name-and-replace.
type convergingChecker struct{}

func (convergingChecker) Check(text string) gate.Result {
	if strings.Count(text, "坚持") >= 3 {
		return gate.Result{Pass: true, NewTokens: 3}
	}
	return gate.Result{Pass: false, Codes: []string{gate.CodeRecurrence},
		Reasons: []string{"needs >= 3"}, NewTokens: 20,
		NewTypeCounts: []gate.NewTypeStat{{ID: 3, Count: 1, InFrontier: true}}}
}

func TestRepairLoopRecordsConvergenceStats(t *testing.T) {
	lex := nameReplaceLex(t)
	prov := &scriptedProvider{texts: []string{"坚持", "坚持坚持", "坚持坚持坚持"}}
	rp := NewReprocessor(prov)
	rp.Lex = lex
	rp.Band = gate.Band{Known: map[int]bool{1: true}, Frontier: map[int]bool{3: true}}

	fate := rp.Run(brief.Brief{CanonID: "c"}, convergingChecker{})
	if !fate.Passed {
		t.Fatal("expected convergence")
	}
	if len(fate.NewTokenTrace) != 3 {
		t.Fatalf("new-token trace = %v, want 3 entries", fate.NewTokenTrace)
	}
	// The trace should end at the passing NewTokens (3) after starting higher (20).
	if fate.NewTokenTrace[0] != 20 || fate.NewTokenTrace[len(fate.NewTokenTrace)-1] != 3 {
		t.Errorf("trace did not show convergence: %v", fate.NewTokenTrace)
	}
}
