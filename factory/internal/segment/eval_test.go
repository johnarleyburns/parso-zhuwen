package segment

import "testing"

func TestEvalCoverageCountsLiterals(t *testing.T) {
	// Dict has 山 and 水; 火 is out-of-dictionary → a literal.
	seg := New(map[string]int{"山": 1, "水": 2}, nil)
	rep := seg.Eval([]string{"山水火。"}, 10)

	if rep.TotalTokens != 3 {
		t.Fatalf("total tokens = %d, want 3", rep.TotalTokens)
	}
	if rep.WordTokens != 2 || rep.LiteralTokens != 1 {
		t.Errorf("word=%d literal=%d, want 2/1", rep.WordTokens, rep.LiteralTokens)
	}
	if rep.DistinctTypes != 2 {
		t.Errorf("distinct types = %d, want 2", rep.DistinctTypes)
	}
	if rep.LiteralRate <= 0 {
		t.Errorf("literal rate = %f, want > 0", rep.LiteralRate)
	}
}

func TestEvalFlagsFMMAmbiguityHotspot(t *testing.T) {
	// Classic FMM overlap: 研究生命 → FMM greedily takes 研究生, but 生命 also starts inside it.
	seg := New(map[string]int{"研究": 1, "研究生": 2, "生命": 3}, nil)
	rep := seg.Eval([]string{"研究生命。"}, 10)

	if len(rep.Hotspots) != 1 {
		t.Fatalf("hotspots = %d, want 1 (%+v)", len(rep.Hotspots), rep.Hotspots)
	}
	h := rep.Hotspots[0]
	if h.Chosen != "研究生" || h.Overlap != "生命" {
		t.Errorf("hotspot = chose %q overlaps %q, want 研究生 / 生命", h.Chosen, h.Overlap)
	}
}

func TestEvalNoHotspotWhenUnambiguous(t *testing.T) {
	seg := New(map[string]int{"山": 1, "水": 2}, nil)
	rep := seg.Eval([]string{"山水山水。"}, 10)
	if len(rep.Hotspots) != 0 {
		t.Errorf("hotspots = %d, want 0", len(rep.Hotspots))
	}
	if rep.TokenCoverage != 1.0 {
		t.Errorf("coverage = %f, want 1.0", rep.TokenCoverage)
	}
}
