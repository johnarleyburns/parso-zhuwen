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
