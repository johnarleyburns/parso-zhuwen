// Package canon holds the source-canon registry (00 §6A, handoff §4.2). Every narrative
// story derives from a registered public-domain source; entries carry provenance and a
// required pd_rationale (FR-12.2). CP-01 seeds 10 entries (00 §6A.1).
package canon

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Character is a named entity with a fixed Chinese name and an English gloss.
type Character struct {
	NameZH string `json:"name_zh"`
	Gloss  string `json:"gloss"`
}

// Entry is one canon registry record.
type Entry struct {
	CanonID       string      `json:"canon_id"`
	Tier          string      `json:"tier"` // C1..C7
	TitleZH       string      `json:"title_zh"`
	TitleEN       string      `json:"title_en"`
	SourceURLs    []string    `json:"source_urls"`
	PDRationale   string      `json:"pd_rationale"` // required (FR-12.2)
	Beats         []string    `json:"beats"`
	Characters    []Character `json:"characters"`
	CulturalNotes []string    `json:"cultural_notes"`
	Chengyu       string      `json:"chengyu,omitempty"`
	Origin        string      `json:"origin"` // "canon" | "original"
}

// Registry is an immutable set of canon entries.
type Registry struct {
	entries []Entry
	byID    map[string]Entry
}

// Load reads a JSON array of entries.
func Load(r io.Reader) (*Registry, error) {
	var entries []Entry
	dec := json.NewDecoder(r)
	if err := dec.Decode(&entries); err != nil {
		return nil, fmt.Errorf("canon: decode: %w", err)
	}
	reg := &Registry{byID: map[string]Entry{}}
	for _, e := range entries {
		if _, dup := reg.byID[e.CanonID]; dup {
			return nil, fmt.Errorf("canon: duplicate canon_id %q", e.CanonID)
		}
		reg.entries = append(reg.entries, e)
		reg.byID[e.CanonID] = e
	}
	if err := reg.Validate(); err != nil {
		return nil, err
	}
	return reg, nil
}

// LoadFile reads a registry JSON file.
func LoadFile(path string) (*Registry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// All returns entries in file order.
func (r *Registry) All() []Entry { return r.entries }

// Get returns the entry with the given canon ID.
func (r *Registry) Get(id string) (Entry, bool) {
	e, ok := r.byID[id]
	return e, ok
}

// Len returns the number of entries.
func (r *Registry) Len() int { return len(r.entries) }

// Validate enforces the registry rules (FR-12.2): every entry needs a canon_id, tier,
// title, at least one beat and source, an origin, and a non-empty pd_rationale.
func (r *Registry) Validate() error {
	for _, e := range r.entries {
		switch {
		case e.CanonID == "":
			return fmt.Errorf("canon: entry missing canon_id")
		case e.Tier == "":
			return fmt.Errorf("canon %s: missing tier", e.CanonID)
		case e.TitleZH == "":
			return fmt.Errorf("canon %s: missing title_zh", e.CanonID)
		case len(e.Beats) == 0:
			return fmt.Errorf("canon %s: no beats", e.CanonID)
		case len(e.SourceURLs) == 0:
			return fmt.Errorf("canon %s: no source_urls", e.CanonID)
		case e.PDRationale == "":
			return fmt.Errorf("canon %s: missing pd_rationale (FR-12.2)", e.CanonID)
		case e.Origin != "canon" && e.Origin != "original":
			return fmt.Errorf("canon %s: invalid origin %q", e.CanonID, e.Origin)
		}
	}
	return nil
}
