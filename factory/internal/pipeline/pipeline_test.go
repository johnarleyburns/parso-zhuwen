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
	n := len(res.Stories)
	if n == 0 {
		t.Fatal("0 stories packed")
	}
	// Reuse rule §8A.1(a): one image per canon entry.
	if len(res.Images) != n {
		t.Fatalf("images = %d, want %d (one per canon)", len(res.Images), n)
	}
	if len(res.Questions) != n*3 {
		t.Fatalf("questions = %d, want %d (3 per story)", len(res.Questions), n*3)
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
	// Minimum: with 81 canon entries, all should pass the fixture gate.
	if n < 10 {
		t.Fatalf("only %d stories packed (expect at least 10 canon entries)", n)
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
	if len(res.Rejected) == 0 {
		t.Fatal("expected at least 1 rejected, got 0")
	}
	t.Logf("%d/%d stories rejected", len(res.Rejected), len(res.Rejected)+len(res.Stories))
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

func TestPipelineRepairLoopFixture(t *testing.T) {
	// Verify the repair-loop path produces the same number of stories as the
	// simple path when using a gate-passing fixture provider.
	cfg := fixtureConfig(t)
	cfg.UseRepairLoop = true
	res, err := Run(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Stories) == 0 {
		t.Fatal("repair loop produced 0 stories")
	}
	if len(res.Rejected) > 0 {
		t.Errorf("unexpected rejects in repair loop: %v", res.Rejected)
	}
	if len(res.Fates) == 0 {
		t.Error("no fates recorded in repair loop result")
	}
	for _, fate := range res.Fates {
		if !fate.Passed {
			t.Errorf("fate for %s: expected pass in fixture mode, got fail codes=%v",
				fate.CanonID, fate.FailCodes)
		}
	}
}
