package gen

import (
	"strings"
	"testing"

	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/lexicon"
)

// constrainedTestLex builds a tiny lexicon: known filler 山(1) 水(2) 人(4); frontier 坚持(3).
func constrainedTestLex(t *testing.T) *lexicon.Lexicon {
	t.Helper()
	lex, err := lexicon.Ingest(strings.NewReader(
		"1\t山\tshān\t1\t10\t\n2\t水\tshuǐ\t1\t11\t\n3\t坚持\tjiānchí\t3\t900\t\n4\t人\trén\t1\t5\t\n"),
		"con-v1")
	if err != nil {
		t.Fatal(err)
	}
	return lex
}

func constrainedTestConstraint(lex *lexicon.Lexicon) GateConstraint {
	return GateConstraint{
		Dict:     lex.DictEntries(),
		Band:     gate.Band{Known: map[int]bool{1: true, 2: true, 4: true}, Frontier: map[int]bool{3: true}, Grammar: map[string]bool{}},
		Detector: grammar.MarkerDetector{},
		MaxID:    lex.MaxID(),
		Cfg:      gate.DefaultConfig(),
	}
}

// scriptedInner returns a fixed sequence of texts, then repeats the last; it counts tokens so
// the ConstrainedProvider's per-story ceiling can be exercised deterministically.
type scriptedInner struct {
	texts    []string
	calls    int
	perCall  int // tokens reported per call
	usedToks int
}

func (s *scriptedInner) Retell(b brief.Brief) (Story, error) {
	i := s.calls
	if i >= len(s.texts) {
		i = len(s.texts) - 1
	}
	s.calls++
	s.usedToks += s.perCall
	return Story{CanonID: b.CanonID, Text: s.texts[i]}, nil
}
func (s *scriptedInner) TokensUsed() int { return s.usedToks }

// passText builds a gate-passing text: 197x known + 3x frontier.
func passText() string {
	return strings.Repeat("山", 197) + strings.Repeat("坚持", 3)
}

// failText builds a gate-failing text: an out-of-lexicon literal blows literal_out_of_lexicon.
func failText() string {
	return strings.Repeat("山", 197) + strings.Repeat("Ω", 5)
}

func TestConstrainedProviderPicksPassingCandidate(t *testing.T) {
	lex := constrainedTestLex(t)
	// Two failing candidates, then a passing one — the rerank must return the passing one.
	inner := &scriptedInner{texts: []string{failText(), failText(), passText()}, perCall: 100}
	cp := NewConstrainedProvider(inner, constrainedTestConstraint(lex), 6, 0)
	b := brief.Brief{CanonID: "c1", Frontier: map[int]bool{3: true}}

	s, err := cp.Retell(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Text != passText() {
		t.Errorf("expected the gate-passing candidate to be selected")
	}
	if cp.StoryCandidates() != 3 {
		t.Errorf("candidates = %d, want 3 (stopped at first pass)", cp.StoryCandidates())
	}
	if !cp.curStoryPassed {
		t.Error("expected curStoryPassed=true")
	}
}

func TestConstrainedProviderReturnsBestFailingWhenNonePass(t *testing.T) {
	lex := constrainedTestLex(t)
	inner := &scriptedInner{texts: []string{failText()}, perCall: 100}
	cp := NewConstrainedProvider(inner, constrainedTestConstraint(lex), 3, 0)
	b := brief.Brief{CanonID: "c1", Frontier: map[int]bool{3: true}}

	s, err := cp.Retell(b)
	if err != nil {
		t.Fatalf("no pass but no ceiling should not error: %v", err)
	}
	if s.Text == "" {
		t.Error("expected a best-effort candidate returned for the repair loop")
	}
	if cp.StoryCandidates() != 3 {
		t.Errorf("candidates = %d, want 3 (full oversample)", cp.StoryCandidates())
	}
}

func TestConstrainedProviderAbortsOnTokenCeiling(t *testing.T) {
	lex := constrainedTestLex(t)
	// Each call costs 100 tokens; ceiling 150 => aborts after the 2nd candidate with no pass.
	inner := &scriptedInner{texts: []string{failText()}, perCall: 100}
	cp := NewConstrainedProvider(inner, constrainedTestConstraint(lex), 10, 150)
	b := brief.Brief{CanonID: "c1", Frontier: map[int]bool{3: true}}

	_, err := cp.Retell(b)
	if err == nil {
		t.Fatal("expected a per-story token-ceiling abort error")
	}
	if !strings.Contains(err.Error(), "ceiling") {
		t.Errorf("error should name the ceiling: %v", err)
	}
	if !cp.StoryAborted() {
		t.Error("expected StoryAborted()=true")
	}
	if cp.StoryCandidates() > 2 {
		t.Errorf("candidates = %d, expected ceiling to stop at ~2", cp.StoryCandidates())
	}
}

// repairInner is a scriptedInner that also satisfies RepairProvider.
type repairInner struct{ scriptedInner }

func (r *repairInner) RetellRepair(b brief.Brief, prior, prompt string) (Story, error) {
	return r.Retell(b)
}

func TestConstrainedProviderPerStoryBudgetResetsAcrossBriefs(t *testing.T) {
	lex := constrainedTestLex(t)
	inner := &repairInner{scriptedInner{texts: []string{passText()}, perCall: 100}}
	cp := NewConstrainedProvider(inner, constrainedTestConstraint(lex), 2, 0)

	if _, err := cp.Retell(brief.Brief{CanonID: "c1", Frontier: map[int]bool{3: true}}); err != nil {
		t.Fatal(err)
	}
	c1 := cp.StoryCandidates()
	if _, err := cp.Retell(brief.Brief{CanonID: "c2", Frontier: map[int]bool{3: true}}); err != nil {
		t.Fatal(err)
	}
	if cp.StoryCandidates() != c1 {
		t.Errorf("per-story candidate counter did not reset on new brief: c1=%d c2=%d", c1, cp.StoryCandidates())
	}
}

func TestBandFixtureProviderDeterministicAndPasses(t *testing.T) {
	lex := constrainedTestLex(t)
	p := NewBandFixtureProvider(func(id int) (string, bool) {
		if w, ok := lex.LookupID(id); ok {
			return w.Simp, true
		}
		return "", false
	})
	b := brief.Brief{
		CanonID: "c1", Band: "A2",
		Known:    map[int]bool{1: true, 2: true, 4: true},
		Frontier: map[int]bool{3: true},
	}
	s1, err := p.Retell(b)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := p.Retell(b)
	if err != nil {
		t.Fatal(err)
	}
	if s1.Text != s2.Text {
		t.Error("band-fixture provider must be deterministic")
	}
	if !s1.Fixture {
		t.Error("band-fixture output must be flagged Fixture:true")
	}
	// Sanity: verify it actually passes the gate.
	con := constrainedTestConstraint(lex)
	cp := NewConstrainedProvider(p, con, 1, 0)
	if _, err := cp.Retell(b); err != nil {
		t.Fatalf("band fixture should pass the gate: %v", err)
	}
	if !cp.curStoryPassed {
		t.Error("band-fixture story did not pass the gate")
	}
}
