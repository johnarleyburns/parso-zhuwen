package pipeline

import (
	"testing"

	"github.com/parso/zhuwen-factory/internal/lexicon"
)

func TestBuildHSKBandPartitionsByLevel(t *testing.T) {
	lex, err := lexicon.FromWords("hsk-test", []lexicon.Word{
		{ID: 1, Simp: "我", Pinyin: "wǒ", HSK: 1, FreqRank: 1},
		{ID: 2, Simp: "你", Pinyin: "nǐ", HSK: 2, FreqRank: 2},
		{ID: 3, Simp: "坚持", Pinyin: "jiānchí", HSK: 3, FreqRank: 3},
		{ID: 4, Simp: "分析", Pinyin: "fēnxī", HSK: 4, FreqRank: 4},
	})
	if err != nil {
		t.Fatal(err)
	}

	// A2: known = HSK 1–2, frontier = HSK 3. HSK-4 words are neither (they'll fail the gate).
	spec := BuildHSKBand(lex, "A2", 2, 3, 2)
	if !spec.Known[1] || !spec.Known[2] {
		t.Error("HSK 1–2 words should be known")
	}
	if spec.Known[3] || spec.Known[4] {
		t.Error("HSK 3/4 words must not be known at A2")
	}
	if !spec.Frontier[3] {
		t.Error("HSK 3 word should be a frontier candidate")
	}
	if spec.Frontier[4] || spec.Frontier[1] {
		t.Error("only the frontier level belongs in the frontier set")
	}
	// 把/被 must be whitelisted now (standard teachable grammar; not an I1 change).
	if !spec.Grammar["ba-construction"] || !spec.Grammar["bei-construction"] {
		t.Error("ba/bei constructions should be whitelisted")
	}
	if spec.Band != "A2" || spec.HSK3Level != 2 {
		t.Errorf("band metadata wrong: %+v", spec)
	}
}
