// Package assets embeds the CP-01 seed fixtures (lexicon + 10-entry canon registry) so
// both the CLI and tests load identical, drift-free data.
package assets

import (
	"bytes"
	_ "embed"

	"github.com/parso/zhuwen-factory/internal/canon"
	"github.com/parso/zhuwen-factory/internal/lexicon"
)

//go:embed lexicon.tsv
var lexiconTSV []byte

//go:embed canon.seed.json
var canonJSON []byte

// FixtureLexiconVersion pins the word IDs of the embedded fixture lexicon.
const FixtureLexiconVersion = "fixture-hsk3.0-v0"

// LexiconBytes returns the raw fixture lexicon TSV.
func LexiconBytes() []byte { return append([]byte(nil), lexiconTSV...) }

// CanonBytes returns the raw fixture canon registry JSON.
func CanonBytes() []byte { return append([]byte(nil), canonJSON...) }

// Lexicon ingests the embedded fixture lexicon.
func Lexicon() (*lexicon.Lexicon, error) {
	return lexicon.Ingest(bytes.NewReader(lexiconTSV), FixtureLexiconVersion)
}

// Canon loads the embedded fixture canon registry.
func Canon() (*canon.Registry, error) {
	return canon.Load(bytes.NewReader(canonJSON))
}

// FillerSimps are the known, single-character, non-combining nouns the fixture
// generator uses to build gate-passing stories.
func FillerSimps() []string {
	return []string{"山", "水", "人", "猫", "狗", "树", "花", "鸟", "云", "星"}
}

// FrontierSimps are the frontier words the fixture band allows as new types.
func FrontierSimps() []string {
	return []string{"坚持", "骄傲"}
}
