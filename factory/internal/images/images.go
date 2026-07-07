// Package images implements the §8A Commons image pipeline: fetch candidates from
// Wikimedia Commons, gate them through the license/AI/resolution filters, and produce
// fully-provenanced pack.Image records for pipelined stories and Foundations cards.
//
// Stage contract per handoff §4.6: fetch (network, behind --live) → gate (hermetic) →
// curate (human-in-the-loop decisions) → process (HEIC encoding, external) → join.
// Every stage exposes workq-referrable refs for resumability.
//
// Hermetic CI path: the gate is a pure function tested with golden fixture tables.
// network stages are gated behind a configurable HTTP client.
package images

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// LicenseKind classifies a Commons license into the §8A gate categories.
type LicenseKind int

const (
	LicenseUnknown LicenseKind = iota
	LicensePD                  // Public Domain or CC0 (preferred)
	LicenseCC0                 // CC0 1.0
	LicenseCCBY                // CC-BY (attribution only)
	LicenseCCBYSA              // CC-BY-SA (share-alike, carry-through required)
	LicenseNC                  // NonCommercial — hard reject
	LicenseND                  // NoDerivs — hard reject
	LicenseGFDL                // GFDL-only — hard reject
	LicenseOther               // Any other license not in the accept set
)

// licenseOK and licenseBad patterns match against the Commons LicenseShortName
// string — normalized, lowercase. Must match the §8A hard-gate list: PD, CC0, CC-BY,
// CC-BY-SA — nothing else.
var (
	licenseOK  = regexp.MustCompile(`^(public domain|pd|cc0|cc[ -]?by([ -]?sa)?([ -]\d[\d.]*)?)`)
	licenseBad = regexp.MustCompile(`(\bnc\b|noncommercial|non-commercial|\bnd\b|noderiv|gfdl|fair use|all rights reserved)`)
)

// ClassifyLicense normalizes a license short-name string into a LicenseKind.
// Checks are ordered deliberately: more specific patterns must be tested before
// broader ones (SA before BY, NC/ND before SA, etc.).
func ClassifyLicense(lic string) LicenseKind {
	lic = strings.TrimSpace(strings.ToLower(lic))
	if lic == "" {
		return LicenseUnknown
	}
	if strings.Contains(lic, "public domain") || lic == "pd" {
		return LicensePD
	}
	if strings.Contains(lic, "cc0") || strings.Contains(lic, "zero") {
		return LicenseCC0
	}
	// NC variants must be checked before CC-BY-SA/CC-BY.
	if strings.Contains(lic, "nc") || strings.Contains(lic, "noncommercial") || strings.Contains(lic, "non-commercial") {
		return LicenseNC
	}
	// ND variants: "nd " context or "no-deriv" / "noderiv".
	if strings.Contains(lic, "nd ") || strings.Contains(lic, "-nd-") || strings.Contains(lic, "-nd.") ||
		strings.Contains(lic, "no-deriv") || strings.Contains(lic, "noderiv") {
		return LicenseND
	}
	if strings.Contains(lic, "gfdl") || strings.Contains(lic, "gnu free documentation") {
		return LicenseGFDL
	}
	// Check SA before BY (CC BY-SA contains "by" as substring).
	if strings.Contains(lic, "sa") || strings.Contains(lic, "share-alike") || strings.Contains(lic, "share alike") {
		return LicenseCCBYSA
	}
	if strings.Contains(lic, "cc-by") || strings.Contains(lic, "cc by") || strings.Contains(lic, "cc-") {
		return LicenseCCBY
	}
	return LicenseOther
}

// IsAcceptable returns true for licenses the §8A hard gate allows (PD, CC0, CC-BY, CC-BY-SA).
func (k LicenseKind) IsAcceptable() bool {
	return k == LicensePD || k == LicenseCC0 || k == LicenseCCBY || k == LicenseCCBYSA
}

// IsPreferred returns true for the preferred license tier (PD / CC0 → no carry-through burden).
func (k LicenseKind) IsPreferred() bool { return k == LicensePD || k == LicenseCC0 }

// IsSA returns true if share-alike carry-through is required (CC-BY-SA).
func (k LicenseKind) IsSA() bool { return k == LicenseCCBYSA }

// GateRejectCode is a machine-readable reason for gate rejection.
type GateRejectCode string

const (
	RejectAICategory    GateRejectCode = "ai_generated"
	RejectLicense       GateRejectCode = "license_not_permitted"
	RejectLowRes        GateRejectCode = "resolution_below_1200px"
	RejectMissingURL    GateRejectCode = "missing_source_url"
	RejectMissingAuthor GateRejectCode = "missing_author"
)

// Candidate is a Commons image candidate with metadata needed by the gate.
type Candidate struct {
	Title      string   // File page title (e.g. "File:Foo.jpg")
	DescURL    string   // Commons description page URL
	ThumbURL   string   // Thumbnail URL
	License    string   // LicenseShortName from extmetadata
	LicenseURL string   // LicenseUrl from extmetadata
	Author     string   // Artist from extmetadata
	W, H       int      // Full image dimensions (pixels)
	Categories []string // Category titles (for AI detection)
}

// GateResult is the outcome of gating a candidate.
type GateResult struct {
	Accepted   bool
	RejectCode GateRejectCode
	RejectWhy  string
	Score      float64
	License    LicenseKind
}

// Gate applies the §8A hard gate to a candidate: license check, AI-category exclusion,
// and the 1200px resolution floor. It also scores accepted candidates for best-of-N
// selection (prefer PD/CC0 > CC-BY > CC-BY-SA; prefer higher resolution).
func Gate(c Candidate) GateResult {
	for _, cat := range c.Categories {
		if strings.Contains(strings.ToLower(cat), "ai-generated") {
			return GateResult{RejectCode: RejectAICategory, RejectWhy: "AI-generated category (I6)", License: LicenseUnknown}
		}
	}
	if c.DescURL == "" {
		return GateResult{RejectCode: RejectMissingURL, RejectWhy: "missing Commons description URL"}
	}
	if c.Author == "" {
		return GateResult{RejectCode: RejectMissingAuthor, RejectWhy: "missing author (attribution required)"}
	}

	kind := ClassifyLicense(c.License)
	lic := strings.TrimSpace(strings.ToLower(c.License))

	if !kind.IsAcceptable() {
		why := fmt.Sprintf("license %q not PD/CC0/CC-BY/CC-BY-SA", c.License)
		if lic == "" {
			why = "missing/ambiguous license"
		}
		return GateResult{RejectCode: RejectLicense, RejectWhy: why, License: kind}
	}
	if min(c.W, c.H) < 1200 {
		return GateResult{RejectCode: RejectLowRes, RejectWhy: fmt.Sprintf("short side %dpx < 1200", min(c.W, c.H)), License: kind}
	}

	score := float64(min(c.W, c.H)) / 1000.0
	switch {
	case kind.IsPreferred():
		score += 30
	case kind.IsSA():
		score += 10
	default:
		score += 20
	}
	return GateResult{Accepted: true, Score: score, License: kind}
}

// GateCandidates applies Gate to each candidate, partitions into accepted (sorted by
// score descending) and rejected, and returns the top pick + alternates + rejects.
func GateCandidates(cands []Candidate) (best *Candidate, alts []Candidate, rejects []GateReject, scores map[string]float64) {
	type scored struct {
		c     Candidate
		score float64
	}
	var accepted []scored
	scores = make(map[string]float64, len(cands))
	for _, c := range cands {
		gr := Gate(c)
		if gr.Accepted {
			accepted = append(accepted, scored{c, gr.Score})
			scores[c.Title] = gr.Score
		} else {
			rejects = append(rejects, GateReject{Title: c.Title, Code: gr.RejectCode, Why: gr.RejectWhy})
		}
	}
	sort.SliceStable(accepted, func(i, j int) bool { return accepted[i].score > accepted[j].score })
	if len(accepted) > 0 {
		best = &accepted[0].c
		for i := 1; i < len(accepted); i++ {
			alts = append(alts, accepted[i].c)
		}
	}
	return
}

// GateReject records a rejected candidate with a machine-readable reason.
type GateReject struct {
	Title string
	Code  GateRejectCode
	Why   string
}

// ImageDecision is a curated image choice for a word, as exported from the review sheet.
type ImageDecision struct {
	Word     string `json:"word"`
	Simp     string `json:"simp,omitempty"`
	Decision string `json:"decision"`
	Set      string `json:"set,omitempty"`
	En       string `json:"en,omitempty"`
	Pinyin   string `json:"pinyin,omitempty"`
	Status   string `json:"status,omitempty"` // "commons" or "custom"
	Custom   bool   `json:"custom,omitempty"`
}
