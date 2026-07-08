package pack

import (
	"path/filepath"
	"strings"
	"testing"
)

// makeAudioStory builds a story with audio bytes of a given size (representative Opus weight).
func makeAudioStory(id string, audioBytes int) Story {
	s := validStory()
	s.ID = id
	s.CoverImageID = "img1"
	s.AudioFile = "audio/" + id + ".opus"
	s.AudioData = make([]byte, audioBytes)
	for i := range s.AudioData {
		s.AudioData[i] = byte(i % 251)
	}
	s.Alignment = []AlignToken{{TokenIdx: 0, T0ms: 0, T1ms: 100}}
	return s
}

// TestMeasureBudgetRealisticPackWithinBudget: an A1+A2-scale pack of real 24 kbps audio +
// HEIC covers stays within NFR-3/NFR-4. ~40 stories × ~3 min × 24 kbps ≈ 540 KB each.
func TestMeasureBudgetRealisticPackWithinBudget(t *testing.T) {
	p := validPack()
	p.Stories = nil
	const perStoryAudio = 540 * 1000 // 3 min of 24 kbps mono Opus
	for i := 0; i < 40; i++ {
		p.Stories = append(p.Stories, makeAudioStory("s"+itoa(i), perStoryAudio))
	}
	// One shared cover image of realistic HEIC weight (~120 KB).
	p.Images[0].Data = make([]byte, 120*1000)

	b, err := MeasureBudget(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckBudget(b); err != nil {
		t.Fatalf("realistic A1+A2 pack must be within budget: %v", err)
	}
	if b.AudioBytes < 40*perStoryAudio {
		t.Fatalf("audio subtotal %d too small", b.AudioBytes)
	}
	if b.OnDiskBytes >= MaxOnDiskBytes {
		t.Fatalf("on-disk %d should be well under %d", b.OnDiskBytes, MaxOnDiskBytes)
	}
}

// TestBuildFailsOnNFR4Breach: an over-budget pack (too much audio) must fail the build.
func TestBuildFailsOnNFR4Breach(t *testing.T) {
	p := validPack()
	p.Stories = nil
	// Enough oversized audio to blow past the 250 MB on-disk ceiling.
	for i := 0; i < 30; i++ {
		p.Stories = append(p.Stories, makeAudioStory("big"+itoa(i), 10*1000*1000)) // 10 MB each = 300 MB
	}
	b, err := MeasureBudget(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckBudget(b); err == nil || !strings.Contains(err.Error(), "NFR-4") {
		t.Fatalf("expected NFR-4 breach, got %v", err)
	}
	// And the full build path must reject it too.
	err = Build(p, filepath.Join(t.TempDir(), "over.zpack"), mustPriv(t))
	if err == nil || !strings.Contains(err.Error(), "NFR-4") {
		t.Fatalf("build must fail on NFR-4 breach, got %v", err)
	}
}

// TestBuildFailsOnNFR3Breach: a single starter pack whose download exceeds 90 MB must fail.
func TestBuildFailsOnNFR3Breach(t *testing.T) {
	p := validPack()
	p.Stories = []Story{makeAudioStory("huge", 100*1000*1000)} // 100 MB single-pack download
	err := Build(p, filepath.Join(t.TempDir(), "over.zpack"), mustPriv(t))
	if err == nil || !strings.Contains(err.Error(), "NFR-3") {
		t.Fatalf("build must fail on NFR-3 breach, got %v", err)
	}
}

// TestMeasureBudgetSubtotals verifies audio/image/sqlite classification.
func TestMeasureBudgetSubtotals(t *testing.T) {
	p := validPack()
	p.Stories = []Story{makeAudioStory("s1", 5000)}
	p.Images[0].Data = make([]byte, 3000)
	b, err := MeasureBudget(p)
	if err != nil {
		t.Fatal(err)
	}
	if b.AudioBytes != 5000 {
		t.Fatalf("audio = %d, want 5000", b.AudioBytes)
	}
	if b.ImageBytes != 3000 {
		t.Fatalf("image = %d, want 3000", b.ImageBytes)
	}
	if b.SQLiteBytes <= 0 {
		t.Fatal("sqlite bytes must be counted")
	}
	if b.DownloadBytes < b.OnDiskBytes {
		t.Fatal("download must include on-disk + overhead")
	}
}

// TestValidPackBuildsWithinBudget: the ordinary fixture pack builds green (no false breach).
func TestValidPackBuildsWithinBudget(t *testing.T) {
	if err := Build(validPack(), filepath.Join(t.TempDir(), "ok.zpack"), mustPriv(t)); err != nil {
		t.Fatalf("valid pack must build within budget: %v", err)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
