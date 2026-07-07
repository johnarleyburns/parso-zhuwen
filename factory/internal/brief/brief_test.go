// MC-4.3 back-fill: internal/brief had no test file. Beat-sheet compilation is the contract
// between the canon registry and the generation stage (§4.2), so it gets a golden test.
package brief

import (
	"reflect"
	"testing"

	"github.com/parso/zhuwen-factory/internal/canon"
)

func testEntry() canon.Entry {
	return canon.Entry{
		CanonID: "c-bamiao", Tier: "C1",
		TitleZH: "拔苗助长", TitleEN: "Pulling Up Seedlings to Help Them Grow",
		Beats:      []string{"农夫嫌禾苗长得慢", "他把禾苗一棵棵拔高", "禾苗全都枯死了"},
		Characters: []canon.Character{{NameZH: "农夫", Gloss: "the farmer"}},
		Origin:     "canon",
	}
}

func testSpec() BandSpec {
	return BandSpec{
		Band:      "A2",
		Known:     map[int]bool{1: true, 2: true, 3: true},
		Frontier:  map[int]bool{42: true},
		Grammar:   map[string]bool{"le-aspect": true},
		LengthMin: 120, LengthMax: 400,
		Register: "narrative", HSK3Level: 2,
	}
}

func TestCompileProducesBeatSheetBrief(t *testing.T) {
	e := testEntry()
	spec := testSpec()
	b := Compile(e, spec)

	// Canon identity + narrative content carry through verbatim.
	if b.CanonID != e.CanonID || b.TitleZH != e.TitleZH || b.TitleEN != e.TitleEN {
		t.Errorf("identity not carried: %+v", b)
	}
	if !reflect.DeepEqual(b.Beats, e.Beats) {
		t.Errorf("beats = %v, want %v", b.Beats, e.Beats)
	}
	if !reflect.DeepEqual(b.Characters, e.Characters) {
		t.Errorf("characters = %v, want %v", b.Characters, e.Characters)
	}

	// Band envelope (the lexical constraint the gate will enforce) comes from the spec.
	if b.Band != "A2" || b.Register != "narrative" || b.HSK3Level != 2 {
		t.Errorf("band envelope wrong: %+v", b)
	}
	if b.LengthMin != 120 || b.LengthMax != 400 {
		t.Errorf("length band = %d-%d, want 120-400", b.LengthMin, b.LengthMax)
	}
	if !reflect.DeepEqual(b.Known, spec.Known) || !reflect.DeepEqual(b.Frontier, spec.Frontier) {
		t.Errorf("lexicon slice not passed through")
	}
	if !reflect.DeepEqual(b.Grammar, spec.Grammar) {
		t.Errorf("grammar whitelist not passed through")
	}
}

func TestCompileIsDeterministic(t *testing.T) {
	e, spec := testEntry(), testSpec()
	if !reflect.DeepEqual(Compile(e, spec), Compile(e, spec)) {
		t.Error("Compile is not deterministic")
	}
}

func TestCompileWithPlan(t *testing.T) {
	e := testEntry()
	spec := testSpec()
	plan := []int{42}
	b := CompileWithPlan(e, spec, plan, 4)
	if !reflect.DeepEqual(b.PlanNewTypes, plan) {
		t.Errorf("PlanNewTypes = %v, want %v", b.PlanNewTypes, plan)
	}
	if b.MinRecurrence != 4 {
		t.Errorf("MinRecurrence = %d, want 4", b.MinRecurrence)
	}
}

func TestPickFrontierWordsTruncates(t *testing.T) {
	frontier := map[int]bool{10: true, 3: true, 7: true, 1: true, 5: true}
	picked := PickFrontierWords(frontier, 3)
	if len(picked) != 3 {
		t.Fatalf("expected 3 picked, got %d: %v", len(picked), picked)
	}
	if picked[0] != 1 || picked[1] != 3 || picked[2] != 5 {
		t.Errorf("PickFrontierWords not sorted by ID: %v", picked)
	}
}

func TestPickFrontierWordsSmallerThanK(t *testing.T) {
	frontier := map[int]bool{7: true, 3: true}
	picked := PickFrontierWords(frontier, 5)
	if len(picked) != 2 {
		t.Errorf("expected 2 picked, got %d", len(picked))
	}
}

func TestPickFrontierWordsEmpty(t *testing.T) {
	if picked := PickFrontierWords(nil, 4); picked != nil {
		t.Errorf("expected nil from nil frontier, got %v", picked)
	}
	if picked := PickFrontierWords(map[int]bool{}, 4); picked != nil {
		t.Errorf("expected nil from empty frontier, got %v", picked)
	}
}

func TestCompilePlanNewTypesDefault(t *testing.T) {
	b := Compile(testEntry(), testSpec())
	if b.PlanNewTypes != nil {
		t.Errorf("default PlanNewTypes should be nil, got %v", b.PlanNewTypes)
	}
	if b.MinRecurrence != 3 {
		t.Errorf("default MinRecurrence should be 3, got %d", b.MinRecurrence)
	}
}

func TestSimpsForIDs(t *testing.T) {
	lookup := func(id int) (string, bool) {
		m := map[int]string{1: "一", 42: "水", 100: "火"}
		s, ok := m[id]
		return s, ok
	}
	got := SimpsForIDs([]int{1, 42, 99}, lookup)
	want := []string{"一", "水"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SimpsForIDs = %v, want %v", got, want)
	}
}

func TestFrontierSimps(t *testing.T) {
	lookup := func(id int) (string, bool) {
		m := map[int]string{10: "日", 3: "月", 7: "山"}
		s, ok := m[id]
		return s, ok
	}
	got := FrontierSimps(map[int]bool{10: true, 7: true, 3: true}, lookup)
	want := []string{"月", "山", "日"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FrontierSimps = %v, want %v", got, want)
	}
}
