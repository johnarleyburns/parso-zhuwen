package repair

import (
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/segment"
)

// PipelineChecker is the real segment→gate step used by the repair loop and the spike. It
// always runs the gate on factory segmentation (§4.3) so the loop measures exactly what ships.
type PipelineChecker struct {
	Seg      *segment.Segmenter
	Band     gate.Band
	Detector grammar.Detector
	MaxID    int
	Cfg      gate.Config
	// Dict, when set, lets callers rebuild the segmenter per brief with that brief's
	// proper-noun dictionary (see ForBrief). Seg is used as-is when Dict is nil.
	Dict map[string]int
}

// ForBrief returns a copy of the checker whose segmenter is rebuilt with the brief's
// proper-noun dictionary, so declared names segment as ProperNoun and are excluded from the
// coverage denominator. Requires Dict to be set; otherwise returns the checker unchanged.
func (c PipelineChecker) ForBrief(propers map[string]string) PipelineChecker {
	if c.Dict != nil {
		c.Seg = segment.New(c.Dict, propers)
	}
	return c
}

// Check segments and gates the text, returning the gate Result (with stable failure codes).
func (c PipelineChecker) Check(text string) gate.Result {
	det := c.Detector
	if det == nil {
		det = grammar.MarkerDetector{}
	}
	tokens := c.Seg.Segment(text)
	return gate.Evaluate(tokens, c.Band, det, c.MaxID, c.Cfg)
}
