package gate_test

// This external test documents invariant I1: only gate.Evaluate can produce a populated
// StoryCandidate. External packages can name the type but can only ever hold the zero
// value — its fields are unexported, so a hand-built candidate carries no coverage proof.

import (
	"testing"

	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/segment"
)

func TestStoryCandidateZeroValueCarriesNoProof(t *testing.T) {
	var c gate.StoryCandidate // the only construction available outside the package
	if c.Coverage() != 0 || c.CoverageBitmap() != nil || c.NewTypeIDs() != nil ||
		c.TokenCount() != 0 || c.TypeCount() != 0 || c.Tokens() != nil {
		t.Fatal("zero-value StoryCandidate must be empty (I1 private-init)")
	}
}

func TestEvaluateIsTheOnlyProducer(t *testing.T) {
	toks := make([]segment.Token, 0, 200)
	for i := 0; i < 200; i++ {
		toks = append(toks, segment.Token{Text: "山", WordID: 1, Kind: segment.Word})
	}
	r := gate.Evaluate(toks, gate.Band{Known: map[int]bool{1: true}, Frontier: map[int]bool{}, Grammar: map[string]bool{}},
		grammar.MarkerDetector{}, 1, gate.DefaultConfig())
	if !r.Pass || r.Candidate == nil || r.Candidate.Coverage() != 1.0 {
		t.Fatalf("Evaluate should produce a full-coverage candidate, got pass=%v", r.Pass)
	}
}
