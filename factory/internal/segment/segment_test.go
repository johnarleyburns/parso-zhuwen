package segment

import "testing"

func newSeg() *Segmenter {
	dict := map[string]int{"山": 1, "水": 2, "坚持": 11, "睡觉": 50, "睡": 51}
	propers := map[string]string{"后羿": "Hou Yi"}
	return New(dict, propers)
}

func TestForwardMaximumMatching(t *testing.T) {
	// 睡觉 must win over 睡 (longest match).
	toks := newSeg().Segment("睡觉")
	if len(toks) != 1 || toks[0].Text != "睡觉" || toks[0].WordID != 50 || toks[0].Kind != Word {
		t.Fatalf("FMM failed: %+v", toks)
	}
}

func TestWordAndSentenceSplitting(t *testing.T) {
	toks := newSeg().Segment("山水坚持。山")
	want := []struct {
		text string
		id   int
		sent int
	}{
		{"山", 1, 0}, {"水", 2, 0}, {"坚持", 11, 0}, {"山", 1, 1},
	}
	if len(toks) != len(want) {
		t.Fatalf("got %d tokens, want %d: %+v", len(toks), len(want), toks)
	}
	for i, w := range want {
		if toks[i].Text != w.text || toks[i].WordID != w.id || toks[i].SentenceIdx != w.sent || toks[i].Kind != Word {
			t.Errorf("token %d = %+v, want %+v", i, toks[i], w)
		}
	}
}

func TestProperNounTaggingAndFirst(t *testing.T) {
	toks := newSeg().Segment("后羿山。后羿水")
	if toks[0].Kind != ProperNoun || !toks[0].First || toks[0].Gloss != "Hou Yi" {
		t.Fatalf("first proper noun wrong: %+v", toks[0])
	}
	// second occurrence of 后羿 is not First
	var second *Token
	for i := range toks {
		if toks[i].Text == "后羿" && !toks[i].First {
			second = &toks[i]
		}
	}
	if second == nil {
		t.Fatal("expected a non-first proper noun occurrence")
	}
}

func TestLiteralFallbackAndSkippable(t *testing.T) {
	// 太 is not in the dict -> literal; comma is skipped.
	toks := newSeg().Segment("山，太")
	if len(toks) != 2 {
		t.Fatalf("got %d tokens: %+v", len(toks), toks)
	}
	if toks[1].Kind != Literal || toks[1].WordID != -1 || toks[1].Text != "太" {
		t.Errorf("literal token wrong: %+v", toks[1])
	}
}
