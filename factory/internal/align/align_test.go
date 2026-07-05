package align

import (
	"reflect"
	"testing"

	"github.com/parso/zhuwen-factory/internal/segment"
)

func toks(specs ...[2]interface{}) []segment.Token {
	out := make([]segment.Token, 0, len(specs))
	for _, s := range specs {
		out = append(out, segment.Token{Text: s[0].(string), SentenceIdx: s[1].(int)})
	}
	return out
}

func sample() []segment.Token {
	// two sentences: 我 爱 中国 · 你 好
	return toks(
		[2]interface{}{"我", 0}, [2]interface{}{"爱", 0}, [2]interface{}{"中国", 0},
		[2]interface{}{"你", 1}, [2]interface{}{"好", 1},
	)
}

func TestAlignOneRowPerToken(t *testing.T) {
	tokens := sample()
	rows, total := Align(tokens, DefaultConfig())
	if len(rows) != len(tokens) {
		t.Fatalf("rows = %d, want %d", len(rows), len(tokens))
	}
	for i, r := range rows {
		if r.TokenIdx != i {
			t.Fatalf("row %d token_idx = %d", i, r.TokenIdx)
		}
	}
	if total <= rows[len(rows)-1].T1ms {
		t.Fatalf("total %d must exceed last t1 %d (tail)", total, rows[len(rows)-1].T1ms)
	}
}

func TestAlignStrictlyIncreasingNonOverlapping(t *testing.T) {
	rows, _ := Align(sample(), DefaultConfig())
	for i, r := range rows {
		if r.T1ms <= r.T0ms {
			t.Fatalf("row %d has non-positive duration [%d,%d)", i, r.T0ms, r.T1ms)
		}
		if i > 0 && r.T0ms < rows[i-1].T1ms {
			t.Fatalf("row %d overlaps previous: t0=%d < prev t1=%d", i, r.T0ms, rows[i-1].T1ms)
		}
	}
}

func TestAlignSentenceContiguityAndGap(t *testing.T) {
	cfg := DefaultConfig()
	rows, _ := Align(sample(), cfg)
	// Within sentence 0 (indices 0..2) the rows are contiguous.
	if rows[1].T0ms != rows[0].T1ms {
		t.Fatalf("intra-sentence not contiguous: %d != %d", rows[1].T0ms, rows[0].T1ms)
	}
	if rows[2].T0ms != rows[1].T1ms {
		t.Fatalf("intra-sentence not contiguous: %d != %d", rows[2].T0ms, rows[1].T1ms)
	}
	// Sentence boundary between index 2 and 3 inserts exactly SentenceGapMs of silence.
	gap := rows[3].T0ms - rows[2].T1ms
	if gap != cfg.SentenceGapMs {
		t.Fatalf("sentence gap = %d, want %d", gap, cfg.SentenceGapMs)
	}
}

func TestAlignPerCharDuration(t *testing.T) {
	cfg := DefaultConfig()
	rows, _ := Align(sample(), cfg)
	// 中国 is two characters → 2 * PerCharMs.
	if got := rows[2].T1ms - rows[2].T0ms; got != 2*cfg.PerCharMs {
		t.Fatalf("中国 duration = %d, want %d", got, 2*cfg.PerCharMs)
	}
	// 我 is one character; PerCharMs already exceeds the floor.
	if got := rows[0].T1ms - rows[0].T0ms; got != cfg.PerCharMs {
		t.Fatalf("我 duration = %d, want %d", got, cfg.PerCharMs)
	}
}

func TestAlignMinTokenFloor(t *testing.T) {
	cfg := Config{PerCharMs: 40, MinTokenMs: 140, SentenceGapMs: 100, LeadInMs: 0, TailMs: 0}
	rows, _ := Align(toks([2]interface{}{"你", 0}), cfg)
	if got := rows[0].T1ms - rows[0].T0ms; got != cfg.MinTokenMs {
		t.Fatalf("floored duration = %d, want %d", got, cfg.MinTokenMs)
	}
}

func TestAlignDeterministic(t *testing.T) {
	a, ta := Align(sample(), DefaultConfig())
	b, tb := Align(sample(), DefaultConfig())
	if ta != tb || !reflect.DeepEqual(a, b) {
		t.Fatalf("alignment not deterministic")
	}
}

func TestAlignEmpty(t *testing.T) {
	rows, total := Align(nil, DefaultConfig())
	if len(rows) != 0 {
		t.Fatalf("expected no rows, got %d", len(rows))
	}
	if total < 0 {
		t.Fatalf("total must be non-negative, got %d", total)
	}
}
