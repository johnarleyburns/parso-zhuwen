package gen

import (
	"strings"
	"testing"

	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/lexicon"
)

// MC-4.3 back-fill: the fixture provider's determinism has always been a design property
// but was never explicitly tested. Two calls with the same brief produce the same Story.
func TestFixtureProviderIsDeterministic(t *testing.T) {
	lex, err := lexicon.Ingest(strings.NewReader(
		"1\t山\tshān\t1\t10\t\n2\t水\tshuǐ\t1\t11\t\n3\t坚持\tjiānchí\t3\t900\t\n4\t人\trén\t1\t5\t\n"),
		"det-v1")
	if err != nil {
		t.Fatal(err)
	}
	p := NewFixtureProvider(lex, []string{"山", "水", "人"})
	b := brief.Brief{
		CanonID: "c1", TitleZH: "故事", Band: "A2", Register: "narrative",
		Known:    map[int]bool{1: true, 2: true, 4: true},
		Frontier: map[int]bool{3: true},
	}
	s1, err := p.Retell(b)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := p.Retell(b)
	if err != nil {
		t.Fatal(err)
	}
	if s1.Text != s2.Text || s1.Fixture != true {
		t.Error("fixture provider is not deterministic")
	}
}

func TestFixtureProviderRejectsUnknownFiller(t *testing.T) {
	lex, _ := lexicon.Ingest(strings.NewReader("1\t山\tshān\t1\t10\t\n"), "v")
	p := NewFixtureProvider(lex, []string{"山", "星"})
	_, err := p.Retell(brief.Brief{Known: map[int]bool{1: true}})
	if err == nil {
		t.Fatal("expected rejection of filler not in lexicon")
	}
}
