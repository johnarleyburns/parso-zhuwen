package pipeline

import (
	"testing"

	"github.com/parso/zhuwen-factory/internal/assets"
	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/gen"
)

func fixtureConfig(t *testing.T) Config {
	t.Helper()
	lex, err := assets.Lexicon()
	if err != nil {
		t.Fatal(err)
	}
	reg, err := assets.Canon()
	if err != nil {
		t.Fatal(err)
	}
	band, err := BuildFixtureBand(lex, assets.FrontierSimps())
	if err != nil {
		t.Fatal(err)
	}
	return Config{
		Lexicon:  lex,
		Registry: reg,
		Band:     band,
		Provider: gen.NewFixtureProvider(lex, assets.FillerSimps()),
		GateCfg:  gate.DefaultConfig(),
	}
}

func TestPipelineAllSeedsPassGate(t *testing.T) {
	res, err := Run(fixtureConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rejected) != 0 {
		t.Fatalf("unexpected rejects: %v", res.Rejected)
	}
	if len(res.Stories) != 10 {
		t.Fatalf("packed %d stories, want 10", len(res.Stories))
	}
	// Reuse rule §8A.1(a): one image per canon entry.
	if len(res.Images) != 10 {
		t.Fatalf("images = %d, want 10 (one per canon)", len(res.Images))
	}
	if len(res.Questions) != 30 {
		t.Fatalf("questions = %d, want 30 (3 per story)", len(res.Questions))
	}
	for _, s := range res.Stories {
		if s.CoverImageID == "" {
			t.Errorf("story %s missing cover image (I6)", s.ID)
		}
		if len(s.Body) == 0 {
			t.Errorf("story %s has empty body", s.ID)
		}
		if s.TokenCount < 150 {
			t.Errorf("story %s tokenCount %d too small for <=2%% new", s.ID, s.TokenCount)
		}
		if len(s.NewTypeIDs) == 0 || len(s.NewTypeIDs) > 8 {
			t.Errorf("story %s newTypeIDs = %v (want 1..8)", s.ID, s.NewTypeIDs)
		}
		if s.Origin != "canon" {
			t.Errorf("story %s origin = %q", s.ID, s.Origin)
		}
	}
}

// badProvider emits a story whose single new word occurs only once (recurrence < 3).
type badProvider struct{}

func (badProvider) Retell(b brief.Brief) (gen.Story, error) {
	// 山 (known) x5 then 坚持 once then 。 -> 坚持 fails recurrence.
	return gen.Story{CanonID: b.CanonID, TitleZH: b.TitleZH, Band: b.Band, Text: "山山山山山坚持。", Fixture: true}, nil
}

func TestPipelineRejectsGateViolation(t *testing.T) {
	cfg := fixtureConfig(t)
	cfg.Provider = badProvider{}
	res, err := Run(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Stories) != 0 {
		t.Errorf("expected 0 packed stories, got %d", len(res.Stories))
	}
	if len(res.Rejected) != 10 {
		t.Fatalf("expected 10 rejected, got %d", len(res.Rejected))
	}
	foundRecurrence := false
	for _, r := range res.Rejected {
		for _, reason := range r.Reasons {
			if len(reason) > 0 && (contains(reason, "needs >= 3") || contains(reason, "ratio")) {
				foundRecurrence = true
			}
		}
	}
	if !foundRecurrence {
		t.Errorf("expected recurrence/ratio rejection reasons: %v", res.Rejected)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
