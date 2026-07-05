// Package brief compiles a canon entry into a constrained retelling brief
// (handoff §4.2): a beat sheet plus the target lexicon slice (allowed word IDs =
// band lexicon; frontier candidates marked), length band, and register.
package brief

import "github.com/parso/zhuwen-factory/internal/canon"

// BandSpec is the lexical envelope for a band (e.g. A2): the known word-ID slice,
// the frontier-candidate word IDs the gate will allow as new types, and the grammar
// whitelist for the band.
type BandSpec struct {
	Band      string
	Known     map[int]bool
	Frontier  map[int]bool
	Grammar   map[string]bool
	LengthMin int
	LengthMax int
	Register  string
	HSK3Level int
}

// Brief is the compiled instruction handed to the generation stage.
type Brief struct {
	CanonID    string
	TitleZH    string
	TitleEN    string
	Band       string
	Register   string
	HSK3Level  int
	Beats      []string
	Characters []canon.Character
	Known      map[int]bool
	Frontier   map[int]bool
	Grammar    map[string]bool
	LengthMin  int
	LengthMax  int
}

// Compile builds a Brief from a canon entry and a band spec.
func Compile(e canon.Entry, spec BandSpec) Brief {
	return Brief{
		CanonID:    e.CanonID,
		TitleZH:    e.TitleZH,
		TitleEN:    e.TitleEN,
		Band:       spec.Band,
		Register:   spec.Register,
		HSK3Level:  spec.HSK3Level,
		Beats:      e.Beats,
		Characters: e.Characters,
		Known:      spec.Known,
		Frontier:   spec.Frontier,
		Grammar:    spec.Grammar,
		LengthMin:  spec.LengthMin,
		LengthMax:  spec.LengthMax,
	}
}
