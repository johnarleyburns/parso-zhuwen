package foundations

import (
	"testing"

	"github.com/parso/zhuwen-factory/internal/images"
	"github.com/parso/zhuwen-factory/internal/lexicon"
)

func testLexicon() *lexicon.Lexicon {
	lex, _ := lexicon.FromWords("test-v1", []lexicon.Word{
		{ID: 1, Simp: "狗", Pinyin: "gǒu", HSK: 1, FreqRank: 500},
		{ID: 2, Simp: "猫", Pinyin: "māo", HSK: 1, FreqRank: 600},
		{ID: 3, Simp: "鱼", Pinyin: "yú", HSK: 2, FreqRank: 800},
		{ID: 4, Simp: "鸟", Pinyin: "niǎo", HSK: 2, FreqRank: 900},
		{ID: 5, Simp: "水", Pinyin: "shuǐ", HSK: 1, FreqRank: 200},
		{ID: 6, Simp: "茶", Pinyin: "chá", HSK: 1, FreqRank: 700},
		{ID: 7, Simp: "火", Pinyin: "huǒ", HSK: 2, FreqRank: 400},
	})
	return lex
}

func testProv() images.ProvenanceStore {
	return images.ProvenanceStore{
		"File:Dog.jpg":   {File: "File:Dog.jpg", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", Author: "A", SourceURL: "https://commons.example/Dog", RetrievedAt: "2026-07-01", W: 2000, H: 1500},
		"File:Cat.jpg":   {File: "File:Cat.jpg", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", Author: "B", SourceURL: "https://commons.example/Cat", RetrievedAt: "2026-07-01", W: 2000, H: 1500},
		"File:Fish.jpg":  {File: "File:Fish.jpg", License: "CC-BY 4.0", LicenseURL: "https://creativecommons.org/licenses/by/4.0/", Author: "C", SourceURL: "https://commons.example/Fish", RetrievedAt: "2026-07-01", W: 2000, H: 1500},
		"File:Bird.jpg":  {File: "File:Bird.jpg", License: "CC-BY 4.0", LicenseURL: "https://creativecommons.org/licenses/by/4.0/", Author: "D", SourceURL: "https://commons.example/Bird", RetrievedAt: "2026-07-01", W: 2000, H: 1500},
		"File:Water.jpg": {File: "File:Water.jpg", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", Author: "E", SourceURL: "https://commons.example/Water", RetrievedAt: "2026-07-01", W: 2000, H: 1500},
		"File:Tea.jpg":   {File: "File:Tea.jpg", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", Author: "F", SourceURL: "https://commons.example/Tea", RetrievedAt: "2026-07-01", W: 2000, H: 1500},
		"File:Fire.jpg":  {File: "File:Fire.jpg", License: "Public Domain", LicenseURL: "https://creativecommons.org/publicdomain/mark/1.0/", Author: "G", SourceURL: "https://commons.example/Fire", RetrievedAt: "2026-07-01", W: 2000, H: 1500},
	}
}

func TestBuildCards(t *testing.T) {
	lex := testLexicon()
	prov := testProv()
	decisions := []images.ImageDecision{
		{Simp: "狗", Decision: "File:Dog.jpg", Set: "animals", Status: "commons"},
		{Simp: "猫", Decision: "File:Cat.jpg", Set: "animals", Status: "commons"},
		{Simp: "鱼", Decision: "File:Fish.jpg", Set: "animals", Status: "commons"},
		{Simp: "鸟", Decision: "File:Bird.jpg", Set: "animals", Status: "commons"},
		{Simp: "水", Decision: "File:Water.jpg", Set: "food-drink", Status: "commons"},
		{Simp: "茶", Decision: "File:Tea.jpg", Set: "food-drink", Status: "commons"},
		{Simp: "火", Decision: "File:Fire.jpg", Set: "nature", Status: "commons"},
	}

	cfg := Config{Lexicon: lex, Decisions: decisions, Prov: prov, MinCardinality: 2}
	cards, sets, err := BuildCards(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// 7 decisions, all have provenances → 7 cards.
	if len(cards) != 7 {
		t.Fatalf("got %d cards, want 7", len(cards))
	}

	// "nature" set has only 1 card < MinCardinality(2) → excluded from sets.
	if len(sets) != 2 {
		t.Fatalf("got %d sets, want 2 (animals, food-drink; nature excluded)", len(sets))
	}

	if sets[0].ID != "animals" {
		t.Errorf("first set = %q, want animals", sets[0].ID)
	}
	if len(sets[0].Cards) != 4 {
		t.Errorf("animals set has %d cards, want 4", len(sets[0].Cards))
	}
	if sets[1].ID != "food-drink" {
		t.Errorf("second set = %q, want food-drink", sets[1].ID)
	}
}

func TestBuildCardsMissingProvenance(t *testing.T) {
	lex := testLexicon()
	prov := images.ProvenanceStore{} // empty
	decisions := []images.ImageDecision{
		{Simp: "狗", Decision: "File:Dog.jpg", Set: "animals", Status: "commons"},
		{Simp: "猫", Decision: "File:Cat.jpg", Set: "animals", Status: "commons"},
	}

	cfg := Config{Lexicon: lex, Decisions: decisions, Prov: prov, MinCardinality: 1}
	cards, _, err := BuildCards(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 0 {
		t.Errorf("got %d cards with empty provenances, want 0", len(cards))
	}
}

func TestBuildCardsMissingLexicon(t *testing.T) {
	cfg := Config{Lexicon: nil}
	_, _, err := BuildCards(cfg)
	if err == nil {
		t.Error("expected error for nil lexicon")
	}
}

func TestAddDistractors(t *testing.T) {
	lex := testLexicon()
	prov := testProv()
	decisions := []images.ImageDecision{
		{Simp: "狗", Decision: "File:Dog.jpg", Set: "animals", Status: "commons"},
		{Simp: "猫", Decision: "File:Cat.jpg", Set: "animals", Status: "commons"},
		{Simp: "鱼", Decision: "File:Fish.jpg", Set: "animals", Status: "commons"},
		{Simp: "鸟", Decision: "File:Bird.jpg", Set: "animals", Status: "commons"},
	}

	cfg := Config{Lexicon: lex, Decisions: decisions, Prov: prov, MinCardinality: 1}
	cards, sets, err := BuildCards(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Group cards by set for distractor selection.
	setCards := map[string][]Card{}
	for _, s := range sets {
		setCards[s.ID] = s.Cards
	}

	AddDistractors(cards, setCards, 3)

	// First card (狗) should have 0 distractors (no predecessors).
	if len(cards[0].DistractorIDs) != 0 {
		t.Errorf("狗 distractors = %v, want [] (first card, no predecessors)", cards[0].DistractorIDs)
	}

	// Second card (猫) should have 1 distractor (狗).
	if len(cards[1].DistractorIDs) != 1 {
		t.Errorf("猫 distractors = %d items, want 1", len(cards[1].DistractorIDs))
	}
	if cards[1].DistractorIDs[0] != 1 { // 狗
		t.Errorf("猫 distractor = %d, want 1 (狗)", cards[1].DistractorIDs[0])
	}

	// Fourth card (鸟) should have 3 distractors from the 3 predecessors.
	if len(cards[3].DistractorIDs) != 3 {
		t.Errorf("鸟 distractors = %d items, want 3", len(cards[3].DistractorIDs))
	}
}

func TestAddDistractorsSmallPool(t *testing.T) {
	lex := testLexicon()
	prov := testProv()
	decisions := []images.ImageDecision{
		{Simp: "狗", Decision: "File:Dog.jpg", Set: "animals", Status: "commons"},
		{Simp: "猫", Decision: "File:Cat.jpg", Set: "animals", Status: "commons"},
	}

	cfg := Config{Lexicon: lex, Decisions: decisions, Prov: prov, MinCardinality: 1}
	cards, _, err := BuildCards(cfg)
	if err != nil {
		t.Fatal(err)
	}

	setCards := map[string][]Card{"animals": cards}
	AddDistractors(cards, setCards, 3)

	// Second card has only 1 predecessor.
	if len(cards[1].DistractorIDs) != 1 {
		t.Errorf("猫 distractors = %d items, want 1 (small pool)", len(cards[1].DistractorIDs))
	}
}

func TestHandoffCheck(t *testing.T) {
	tests := []struct {
		gated int
		ready bool
	}{
		{0, false},
		{10, false},
		{19, false},
		{20, true},
		{50, true},
	}
	for _, tt := range tests {
		hs := CheckHandoff(tt.gated)
		if hs.Ready != tt.ready {
			t.Errorf("CheckHandoff(%d).Ready = %v, want %v", tt.gated, hs.Ready, tt.ready)
		}
		if hs.StoriesGated != tt.gated {
			t.Errorf("CheckHandoff(%d).StoriesGated = %d", tt.gated, hs.StoriesGated)
		}
		if hs.Threshold != HandoffThreshold {
			t.Errorf("CheckHandoff(%d).Threshold = %d, want %d", tt.gated, hs.Threshold, HandoffThreshold)
		}
	}
}

func TestWordImageability(t *testing.T) {
	wi := WordImageability{"狗": 5, "猫": 3, "山": 0}
	if !wi.IsImageable("狗") {
		t.Error("狗 should be imageable (5 candidates)")
	}
	if !wi.IsImageable("猫") {
		t.Error("猫 should be imageable (3 candidates)")
	}
	if wi.IsImageable("山") {
		t.Error("山 should NOT be imageable (0 candidates)")
	}
	if wi.IsImageable("unknown") {
		t.Error("unknown word should NOT be imageable")
	}
}

func TestPickDeterministic(t *testing.T) {
	pool := []int{10, 20, 30, 40, 50, 60, 70}

	// Call twice, should get same result.
	r1 := pickDeterministic(pool, 3)
	r2 := pickDeterministic(pool, 3)

	if len(r1) != 3 || len(r2) != 3 {
		t.Fatalf("expected 3 items, got %d and %d", len(r1), len(r2))
	}
	for i := range r1 {
		if r1[i] != r2[i] {
			t.Errorf("deterministic pick differs at pos %d: %d vs %d", i, r1[i], r2[i])
		}
	}
	// Results should be sorted.
	for i := 1; i < len(r1); i++ {
		if r1[i] < r1[i-1] {
			t.Errorf("result not sorted: %v", r1)
		}
	}
}
