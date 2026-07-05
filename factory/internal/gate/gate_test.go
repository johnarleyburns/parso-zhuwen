package gate

import (
	"testing"

	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/segment"
)

// tok builds an in-lexicon Word token.
func tok(id int, text string) segment.Token {
	return segment.Token{Text: text, WordID: id, Kind: segment.Word}
}

// repeat returns n copies of a Word token.
func repeat(id int, text string, n int) []segment.Token {
	out := make([]segment.Token, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, tok(id, text))
	}
	return out
}

func band(known, frontier []int, grammar ...string) Band {
	b := Band{Known: map[int]bool{}, Frontier: map[int]bool{}, Grammar: map[string]bool{}}
	for _, k := range known {
		b.Known[k] = true
	}
	for _, f := range frontier {
		b.Frontier[f] = true
	}
	for _, g := range grammar {
		b.Grammar[g] = true
	}
	return b
}

var det = grammar.MarkerDetector{}

func TestGatePassBaseline(t *testing.T) {
	// 197 known (id 1) + 3 new frontier (id 4). ratio 3/200 = 1.5% <= 2%.
	toks := append(repeat(1, "山", 197), repeat(4, "坚持", 3)...)
	r := Evaluate(toks, band([]int{1}, []int{4}), det, 4, DefaultConfig())
	if !r.Pass {
		t.Fatalf("expected pass, reasons: %v", r.Reasons)
	}
	c := r.Candidate
	if c == nil {
		t.Fatal("pass but nil candidate")
	}
	if len(c.NewTypeIDs()) != 1 || c.NewTypeIDs()[0] != 4 {
		t.Errorf("new types = %v, want [4]", c.NewTypeIDs())
	}
	if c.TokenCount() != 200 || c.TypeCount() != 2 {
		t.Errorf("tokenCount=%d typeCount=%d", c.TokenCount(), c.TypeCount())
	}
	if c.Coverage() < 0.98 {
		t.Errorf("coverage %.4f < 0.98", c.Coverage())
	}
	// bitmap has bits 1 and 4 set only.
	if !bit(c.CoverageBitmap(), 1) || !bit(c.CoverageBitmap(), 4) || bit(c.CoverageBitmap(), 2) {
		t.Error("coverage bitmap wrong")
	}
}

func bit(b []byte, i int) bool {
	idx := i / 8
	return idx < len(b) && b[idx]&(1<<uint(i%8)) != 0
}

func TestGateFailTypeBudget(t *testing.T) {
	// 9 new frontier types (ids 2..10), each 3x, plus known filler.
	toks := repeat(1, "山", 200)
	for id := 2; id <= 10; id++ {
		toks = append(toks, repeat(id, "x", 3)...)
	}
	fr := []int{2, 3, 4, 5, 6, 7, 8, 9, 10}
	r := Evaluate(toks, band([]int{1}, fr), det, 10, DefaultConfig())
	assertFail(t, r, "new type budget exceeded")
}

func TestGateFailTokenBudget979(t *testing.T) {
	// The canonical must-fail fixture: 97.9% coverage (2.1% new tokens).
	toks := append(repeat(1, "山", 979), repeat(2, "坚持", 21)...)
	r := Evaluate(toks, band([]int{1}, []int{2}), det, 2, DefaultConfig())
	assertFail(t, r, "new-token ratio")
	if r.Coverage >= 0.98 {
		t.Errorf("coverage %.4f should be < 0.98", r.Coverage)
	}
}

func TestGateFailRecurrence(t *testing.T) {
	// new frontier word appears only 2x (< 3).
	toks := append(repeat(1, "山", 200), repeat(2, "坚持", 2)...)
	r := Evaluate(toks, band([]int{1}, []int{2}), det, 2, DefaultConfig())
	assertFail(t, r, "needs >= 3")
}

func TestGateFailFrontierDiscipline(t *testing.T) {
	// new word id 2 is NOT a frontier candidate.
	toks := append(repeat(1, "山", 200), repeat(2, "坚持", 3)...)
	r := Evaluate(toks, band([]int{1}, []int{ /* empty */ }), det, 2, DefaultConfig())
	assertFail(t, r, "not in frontier queue")
}

func TestGateFailGrammarWhitelist(t *testing.T) {
	// A known word 把 triggers ba-construction, not in the (empty) whitelist.
	toks := append(repeat(1, "山", 200), tok(2, "把"))
	b := band([]int{1, 2}, []int{}) // both known, no grammar whitelisted
	r := Evaluate(toks, b, det, 2, DefaultConfig())
	assertFail(t, r, "grammar pattern")
}

func TestGateGrammarWhitelistedPasses(t *testing.T) {
	toks := append(repeat(1, "山", 200), tok(2, "把"))
	b := band([]int{1, 2}, []int{}, "ba-construction")
	r := Evaluate(toks, b, det, 2, DefaultConfig())
	if !r.Pass {
		t.Fatalf("expected pass with whitelisted grammar, reasons: %v", r.Reasons)
	}
}

func TestGateFailProperNounWithoutGloss(t *testing.T) {
	toks := append(repeat(1, "山", 200),
		segment.Token{Text: "后羿", WordID: -1, Kind: segment.ProperNoun, First: true, Gloss: ""})
	r := Evaluate(toks, band([]int{1}, []int{}), det, 1, DefaultConfig())
	assertFail(t, r, "lacks first-occurrence gloss")
}

func TestGateProperNounWithGlossExcludedFromDenominator(t *testing.T) {
	toks := append(repeat(1, "山", 200),
		segment.Token{Text: "后羿", WordID: -1, Kind: segment.ProperNoun, First: true, Gloss: "Hou Yi"})
	r := Evaluate(toks, band([]int{1}, []int{}), det, 1, DefaultConfig())
	if !r.Pass {
		t.Fatalf("glossed proper noun should pass, reasons: %v", r.Reasons)
	}
	if r.Candidate.TokenCount() != 200 { // proper noun excluded from denominator
		t.Errorf("tokenCount = %d, want 200 (proper noun excluded)", r.Candidate.TokenCount())
	}
}

func TestGateFailLiteralOutOfLexicon(t *testing.T) {
	toks := append(repeat(1, "山", 200), segment.Token{Text: "太", WordID: -1, Kind: segment.Literal})
	r := Evaluate(toks, band([]int{1}, []int{}), det, 1, DefaultConfig())
	assertFail(t, r, "out-of-lexicon literal")
}

func assertFail(t *testing.T, r Result, wantSubstr string) {
	t.Helper()
	if r.Pass {
		t.Fatalf("expected fail (%q), but passed", wantSubstr)
	}
	if r.Candidate != nil {
		t.Error("failing result must have nil candidate")
	}
	found := false
	for _, reason := range r.Reasons {
		if contains(reason, wantSubstr) {
			found = true
		}
	}
	if !found {
		t.Errorf("reasons %v do not contain %q", r.Reasons, wantSubstr)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
