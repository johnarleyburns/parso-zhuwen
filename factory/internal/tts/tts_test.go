package tts

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/parso/zhuwen-factory/internal/pack"
	"github.com/parso/zhuwen-factory/internal/segment"
)

func timingsFrom(rows [][3]int) []pack.AlignToken {
	out := make([]pack.AlignToken, 0, len(rows))
	for _, r := range rows {
		out = append(out, pack.AlignToken{TokenIdx: r[0], T0ms: r[1], T1ms: r[2]})
	}
	return out
}

func toks(specs ...[2]interface{}) []segment.Token {
	out := make([]segment.Token, 0, len(specs))
	for _, s := range specs {
		out = append(out, segment.Token{Text: s[0].(string), SentenceIdx: s[1].(int)})
	}
	return out
}

func sample() []segment.Token {
	return toks(
		[2]interface{}{"我", 0}, [2]interface{}{"爱", 0}, [2]interface{}{"中国", 0},
		[2]interface{}{"你", 1}, [2]interface{}{"好", 1},
	)
}

func TestStubDeterministic(t *testing.T) {
	a, err := Render(sample(), "story-1", DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(sample(), "story-1", DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.OpusBytes, b.OpusBytes) {
		t.Fatal("stub opus bytes not deterministic for same story")
	}
	if !reflect.DeepEqual(a.Timings, b.Timings) {
		t.Fatal("stub timings not deterministic")
	}
	if a.DurationMs != b.DurationMs {
		t.Fatal("stub duration not deterministic")
	}
}

func TestStubDistinctPerStory(t *testing.T) {
	a, _ := Render(sample(), "story-1", DefaultConfig())
	b, _ := Render(sample(), "story-2", DefaultConfig())
	if bytes.Equal(a.OpusBytes, b.OpusBytes) {
		t.Fatal("distinct story IDs must yield distinct stub bytes")
	}
}

func TestStubTimingsSatisfyContract(t *testing.T) {
	r, err := Render(sample(), "s", DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateTimings(r.Timings, len(sample())); err != nil {
		t.Fatalf("stub timings violate AlignToken contract: %v", err)
	}
}

func TestStubSizeRepresentativeMatches24kbps(t *testing.T) {
	// With RepresentativeStub, a story's bytes weigh a realistic 24 kbps amount so the
	// NFR-4 size-budget test is not theater. Default stub stays compact.
	cfg := DefaultConfig()
	cfg.RepresentativeStub = true
	r, _ := Render(sample(), "s", cfg)
	want := StubByteLen(r.DurationMs, 24000)
	if len(r.OpusBytes) != want {
		t.Fatalf("representative stub bytes = %d, want %d (24 kbps mono for %d ms)", len(r.OpusBytes), want, r.DurationMs)
	}
	// Default (compact) stub is much smaller.
	def, _ := Render(sample(), "s", DefaultConfig())
	if len(def.OpusBytes) >= want {
		t.Fatalf("default stub (%d B) should be smaller than representative (%d B)", len(def.OpusBytes), want)
	}
	// Sanity: byte length grows with duration.
	if StubByteLen(180_000, 24000) <= StubByteLen(60_000, 24000) {
		t.Fatal("stub size must scale with duration")
	}
}

func TestStubProvenance(t *testing.T) {
	r, _ := Render(sample(), "s", DefaultConfig())
	if r.Provenance.Mode != "stub" || r.Provenance.Tool != "cosyvoice-stub" {
		t.Fatalf("provenance = %+v", r.Provenance)
	}
	if r.Provenance.BitrateBps != 24000 {
		t.Fatalf("bitrate = %d, want 24000", r.Provenance.BitrateBps)
	}
}

func TestValidateTimingsRejectsOverlap(t *testing.T) {
	bad := timingsFrom([][3]int{{0, 0, 100}, {1, 50, 200}})
	if err := ValidateTimings(bad, 2); err == nil {
		t.Fatal("expected overlap rejection")
	}
}

func TestValidateTimingsRejectsBadIndex(t *testing.T) {
	bad := timingsFrom([][3]int{{0, 0, 100}, {5, 100, 200}})
	if err := ValidateTimings(bad, 2); err == nil {
		t.Fatal("expected token_idx rejection")
	}
}

func TestValidateTimingsRejectsWrongCount(t *testing.T) {
	rows := timingsFrom([][3]int{{0, 0, 100}})
	if err := ValidateTimings(rows, 2); err == nil {
		t.Fatal("expected wrong-count rejection")
	}
}

func TestRealModeRequiresConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeReal
	if _, err := Render(sample(), "s", cfg); err == nil {
		t.Fatal("ModeReal without PythonBin/Script must error")
	}
}
