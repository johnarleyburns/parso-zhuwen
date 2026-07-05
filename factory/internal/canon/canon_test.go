package canon

import (
	"strings"
	"testing"
)

func TestValidateRequiresPDRationale(t *testing.T) {
	// FR-12.2: pd_rationale is mandatory.
	j := `[{"canon_id":"X","tier":"C1","title_zh":"甲","beats":["b"],
	       "source_urls":["u"],"origin":"canon"}]`
	if _, err := Load(strings.NewReader(j)); err == nil || !strings.Contains(err.Error(), "pd_rationale") {
		t.Fatalf("expected pd_rationale error, got %v", err)
	}
}

func TestValidateRejectsBadOriginAndMissingFields(t *testing.T) {
	cases := map[string]string{
		"bad origin":  `[{"canon_id":"X","tier":"C1","title_zh":"甲","beats":["b"],"source_urls":["u"],"pd_rationale":"r","origin":"invented"}]`,
		"no beats":    `[{"canon_id":"X","tier":"C1","title_zh":"甲","source_urls":["u"],"pd_rationale":"r","origin":"canon"}]`,
		"no source":   `[{"canon_id":"X","tier":"C1","title_zh":"甲","beats":["b"],"pd_rationale":"r","origin":"canon"}]`,
		"no title":    `[{"canon_id":"X","tier":"C1","beats":["b"],"source_urls":["u"],"pd_rationale":"r","origin":"canon"}]`,
		"no canon_id": `[{"tier":"C1","title_zh":"甲","beats":["b"],"source_urls":["u"],"pd_rationale":"r","origin":"canon"}]`,
	}
	for name, j := range cases {
		if _, err := Load(strings.NewReader(j)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestDuplicateCanonID(t *testing.T) {
	j := `[{"canon_id":"X","tier":"C1","title_zh":"甲","beats":["b"],"source_urls":["u"],"pd_rationale":"r","origin":"canon"},
	       {"canon_id":"X","tier":"C1","title_zh":"乙","beats":["b"],"source_urls":["u"],"pd_rationale":"r","origin":"canon"}]`
	if _, err := Load(strings.NewReader(j)); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestValidRegistryGetAndAll(t *testing.T) {
	j := `[{"canon_id":"C5-x","tier":"C5","title_zh":"龟兔赛跑","title_en":"Tortoise and Hare",
	        "beats":["b1","b2"],"source_urls":["u"],"pd_rationale":"Aesop, PD","origin":"canon",
	        "characters":[{"name_zh":"龟","gloss":"tortoise"}]}]`
	reg, err := Load(strings.NewReader(j))
	if err != nil {
		t.Fatal(err)
	}
	if reg.Len() != 1 {
		t.Fatalf("len = %d", reg.Len())
	}
	e, ok := reg.Get("C5-x")
	if !ok || e.Tier != "C5" || len(e.Characters) != 1 {
		t.Errorf("get returned %+v ok=%v", e, ok)
	}
}
