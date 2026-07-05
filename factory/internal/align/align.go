// Package align is the factory forced-alignment stage (handoff §4.7): it assigns every
// body token a word-level [t0_ms, t1_ms) window shipped in the pack `alignment` table and
// `story.alignment`, driving the app's karaoke highlight (FR-5.1) without the app ever
// computing timing (I3). CP-06 uses a deterministic character-rate model; the real
// CosyVoice 3.0 render + forced aligner replaces the timings at CP-09 behind the same
// contract: one strictly-increasing, non-overlapping row per token, contiguous within a
// sentence with a pause between sentences.
package align

import (
	"github.com/parso/zhuwen-factory/internal/pack"
	"github.com/parso/zhuwen-factory/internal/segment"
)

// Config parameterizes the deterministic timing model. Milliseconds throughout.
type Config struct {
	PerCharMs     int // narration time per Han character
	MinTokenMs    int // floor so single-character tokens are still highlightable
	SentenceGapMs int // silence inserted at each sentence boundary
	LeadInMs      int // silence before the first token
	TailMs        int // silence after the last token (part of total duration)
}

// DefaultConfig is a calm ~3.8 char/s narration pace with natural sentence pauses.
func DefaultConfig() Config {
	return Config{PerCharMs: 260, MinTokenMs: 140, SentenceGapMs: 320, LeadInMs: 250, TailMs: 320}
}

// Align produces one alignment row per token (token_idx = position in tokens) and the
// total audio duration in ms. Pure and deterministic: identical input yields identical
// output. Within a sentence the rows are contiguous (t1 of one == t0 of the next); a
// SentenceGapMs pause separates sentences; the total includes lead-in and tail silence.
func Align(tokens []segment.Token, cfg Config) (rows []pack.AlignToken, totalMs int) {
	rows = make([]pack.AlignToken, 0, len(tokens))
	t := cfg.LeadInMs
	prevSent := -1
	for i, tok := range tokens {
		if prevSent >= 0 && tok.SentenceIdx != prevSent {
			t += cfg.SentenceGapMs
		}
		prevSent = tok.SentenceIdx

		n := len([]rune(tok.Text))
		if n < 1 {
			n = 1
		}
		dur := n * cfg.PerCharMs
		if dur < cfg.MinTokenMs {
			dur = cfg.MinTokenMs
		}
		rows = append(rows, pack.AlignToken{TokenIdx: i, T0ms: t, T1ms: t + dur})
		t += dur
	}
	totalMs = t + cfg.TailMs
	return rows, totalMs
}
