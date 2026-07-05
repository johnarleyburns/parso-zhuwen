// Package grammar provides a lightweight, rule-based grammar-pattern detector used by
// the coverage gate's grammar whitelist check (handoff §4.3: "grammar_patterns(text) ⊄
// band_whitelist"). CP-01 detects a small set of high-signal patterns by marker words;
// production may replace the detector without changing the gate contract.
package grammar

import "github.com/parso/zhuwen-factory/internal/segment"

// Detector reports which grammar-pattern IDs appear in a token stream.
type Detector interface {
	Detect(tokens []segment.Token) []string
}

// markerRules maps a pattern ID to the surface marker word that signals it.
var markerRules = []struct {
	id     string
	marker string
}{
	{"ba-construction", "把"},
	{"bei-construction", "被"},
	{"le-aspect", "了"},
	{"guo-aspect", "过"},
	{"de-attributive", "的"},
	{"ma-question", "吗"},
	{"bu-negation", "不"},
	{"zai-progressive", "在"},
}

// MarkerDetector is the default rule-based detector.
type MarkerDetector struct{}

// Detect returns the sorted-by-rule-order set of detected pattern IDs.
func (MarkerDetector) Detect(tokens []segment.Token) []string {
	present := map[string]bool{}
	for _, t := range tokens {
		present[t.Text] = true
	}
	var out []string
	for _, r := range markerRules {
		if present[r.marker] {
			out = append(out, r.id)
		}
	}
	return out
}
