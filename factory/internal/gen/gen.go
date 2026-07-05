// Package gen is the retelling stage (handoff §4.2). Production plugs a Chinese-strong
// LLM behind the Provider interface; the app itself contains NO generation code (I3).
// CP-01 ships a deterministic FixtureProvider so the whole pipeline is testable offline
// (handoff §6 CP-01: fixtures flagged fixture:true).
package gen

import "github.com/parso/zhuwen-factory/internal/brief"

// Story is a raw retelling: the constrained Chinese text plus metadata. Segmentation
// and gating happen downstream so the gate always runs on factory segmentation (§4.3).
type Story struct {
	CanonID  string
	TitleZH  string
	TitleEN  string
	Band     string
	Register string
	Text     string
	Fixture  bool
}

// Provider retells a brief under lexical constraint.
type Provider interface {
	Retell(b brief.Brief) (Story, error)
}
