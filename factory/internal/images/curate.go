package images

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/parso/zhuwen-factory/internal/pack"
)

// --- Decision loading -------------------------------------------------------

// LoadDecisions reads one or more decisions JSON files and returns a deduplicated,
// sorted list. When a word appears in multiple files, the last file wins (allowing
// progressive override: base decisions + overrides).
func LoadDecisions(paths ...string) ([]ImageDecision, error) {
	m := map[string]ImageDecision{}
	var order []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("decisions %s: %w", p, err)
		}
		var recs []ImageDecision
		if err := json.Unmarshal(b, &recs); err != nil {
			return nil, fmt.Errorf("decisions %s: %w", p, err)
		}
		for _, d := range recs {
			key := d.key()
			if _, seen := m[key]; !seen {
				order = append(order, key)
			}
			m[key] = d
		}
	}
	out := make([]ImageDecision, 0, len(order))
	for _, k := range order {
		out = append(out, m[k])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].key() < out[j].key() })
	return out, nil
}

func (d ImageDecision) key() string {
	if d.Simp != "" {
		return d.Simp
	}
	return d.Word
}

// WordKey returns the canonical word key (simp or word field).
func (d ImageDecision) WordKey() string { return d.key() }

// IsCommons returns true if the decision is a Commons File: reference.
func (d ImageDecision) IsCommons() bool {
	if d.Status == "commons" || strings.HasPrefix(d.Decision, "File:") {
		return true
	}
	// Some decisions use "custom" status but still point to Commons/Wikipedia URLs.
	if strings.Contains(d.Decision, "File:") &&
		(strings.Contains(d.Decision, "wikimedia.org") || strings.Contains(d.Decision, "wikipedia.org")) {
		return true
	}
	return false
}

// CommonsTitle extracts the File: name from the decision. For "File:Foo.jpg" it
// returns "File:Foo.jpg"; for a full URL it extracts the File: path component,
// URL-decodes it, and ensures a "File:" prefix so it matches the Commons API format.
func (d ImageDecision) CommonsTitle() string {
	if strings.HasPrefix(d.Decision, "File:") {
		return d.Decision
	}
	// Handle Wikipedia/Commons URLs: extract the File: title.
	for _, prefix := range []string{"/wiki/File:", "/media/File:"} {
		if idx := strings.Index(d.Decision, prefix); idx >= 0 {
			base := d.Decision[idx+len(prefix):]
			// Strip fragment and query.
			if qi := strings.IndexAny(base, "?#"); qi >= 0 {
				base = base[:qi]
			}
			if dec, err := url.PathUnescape(base); err == nil {
				base = dec
			}
			base = strings.TrimSpace(base)
			if strings.HasPrefix(base, "File:") {
				return base
			}
			return "File:" + base
		}
	}
	return d.Decision
}

// CommonsPageURL returns the full Commons file page URL for the decision.
func (d ImageDecision) CommonsPageURL() string {
	if strings.HasPrefix(d.Decision, "http") {
		return d.Decision
	}
	return "https://commons.wikimedia.org/wiki/" + strings.ReplaceAll(d.Decision, " ", "_")
}

// --- Conversion to pack.Image -----------------------------------------------

// Provenance is the full attribution record extracted from Commons extmetadata.
type Provenance struct {
	File        string
	License     string
	LicenseURL  string
	Author      string
	SourceURL   string
	RetrievedAt string
	W, H        int
	// SignedOff records the owner's per-image license verification on the Commons page
	// (CC-BY/SA attribution is a legal obligation, FR-11.2). CP-09c Part D requires this be
	// recorded, not assumed (I4): DecisionsToImagesSignedOff rejects an unsigned cover so a
	// story cannot graduate off its fixture stand-in without a human sign-off.
	SignedOff bool
	SignedBy  string
	SignedAt  string
}

// ProvenanceStore maps Commons File: titles to provenances. In production this is
// populated from the fetch stage's extmetadata cache; in CI it comes from a fixture.
type ProvenanceStore map[string]Provenance

// Lookup returns the provenance for a Commons title, or nil if not found.
func (ps ProvenanceStore) Lookup(title string) *Provenance {
	p, ok := ps[title]
	if !ok {
		return nil
	}
	return &p
}

// DecisionsToImages converts curated decisions + provenances into pack.Image records
// suitable for the pipeline join stage. wordIDMap maps word keys to lexicon word IDs.
func DecisionsToImages(decisions []ImageDecision, prov ProvenanceStore, wordIDMap map[string]int) ([]pack.Image, error) {
	return decisionsToImages(decisions, prov, wordIDMap, false)
}

// DecisionsToImagesSignedOff is the CP-09c Part D ship-readiness path: it additionally
// requires that every chosen image carry a recorded per-image license sign-off (I4/FR-11.2).
// An unsigned image is rejected, so a story/pack cannot graduate off its fixture stand-in
// without a human having verified the license on the Commons page.
func DecisionsToImagesSignedOff(decisions []ImageDecision, prov ProvenanceStore, wordIDMap map[string]int) ([]pack.Image, error) {
	return decisionsToImages(decisions, prov, wordIDMap, true)
}

func decisionsToImages(decisions []ImageDecision, prov ProvenanceStore, wordIDMap map[string]int, requireSignOff bool) ([]pack.Image, error) {
	var imgs []pack.Image
	for _, d := range decisions {
		title := d.CommonsTitle()
		p := prov.Lookup(title)
		if p == nil {
			return nil, fmt.Errorf("image: provenance not found for decision %q (word %s)", title, d.WordKey())
		}
		if requireSignOff && !p.SignedOff {
			return nil, fmt.Errorf("image: %q (word %s) has no recorded license sign-off (FR-11.2/I4)", title, d.WordKey())
		}
		wid := wordIDMap[d.WordKey()]
		if wid == 0 {
			return nil, fmt.Errorf("image: word %q not in lexicon", d.WordKey())
		}
		imgs = append(imgs, pack.Image{
			ID:          fmt.Sprintf("img-foundations-%s", d.WordKey()),
			WordID:      &wid,
			File:        fmt.Sprintf("images/%s@480.heic", fmt.Sprintf("img-foundations-%s", d.WordKey())),
			W:           p.W,
			H:           p.H,
			License:     p.License,
			LicenseURL:  p.LicenseURL,
			Author:      p.Author,
			SourceURL:   p.SourceURL,
			RetrievedAt: p.RetrievedAt,
		})
	}
	return imgs, nil
}

// CanonDecisionsToImages converts canon story-cover decisions (keyed by title_zh via
// canonIDMap: title_zh → canon_id) + provenances into pack.Image records with CanonID set
// (for the join stage's story-cover replacement — not WordID). requireSignOff gates on a
// recorded per-image license sign-off (CP-09c Part D ship-readiness, I4/FR-11.2).
func CanonDecisionsToImages(decisions []ImageDecision, prov ProvenanceStore, canonIDMap map[string]string, requireSignOff bool) ([]pack.Image, error) {
	var imgs []pack.Image
	for _, d := range decisions {
		if d.Decision == "" || d.Decision == "__reject__" {
			continue
		}
		title := d.CommonsTitle()
		p := prov.Lookup(title)
		if p == nil {
			return nil, fmt.Errorf("image: provenance not found for decision %q (story %s)", title, d.WordKey())
		}
		if requireSignOff && !p.SignedOff {
			return nil, fmt.Errorf("image: %q (story %s) has no recorded license sign-off (FR-11.2/I4)", title, d.WordKey())
		}
		if p.License == "" || p.LicenseURL == "" || p.Author == "" || p.SourceURL == "" || p.RetrievedAt == "" {
			return nil, fmt.Errorf("image: %q (story %s) has an incomplete provenance record (I6)", title, d.WordKey())
		}
		canonID, ok := canonIDMap[d.WordKey()]
		if !ok || canonID == "" {
			return nil, fmt.Errorf("image: story %q not in canon inventory", d.WordKey())
		}
		id := "img-" + canonID
		imgs = append(imgs, pack.Image{
			ID:          id,
			CanonID:     canonID,
			File:        fmt.Sprintf("images/%s@480.heic", id),
			W:           p.W,
			H:           p.H,
			License:     p.License,
			LicenseURL:  p.LicenseURL,
			Author:      p.Author,
			SourceURL:   p.SourceURL,
			RetrievedAt: p.RetrievedAt,
		})
	}
	return imgs, nil
}
