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
