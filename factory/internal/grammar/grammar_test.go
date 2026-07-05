package grammar

import (
	"reflect"
	"testing"

	"github.com/parso/zhuwen-factory/internal/segment"
)

func words(texts ...string) []segment.Token {
	var t []segment.Token
	for _, x := range texts {
		t = append(t, segment.Token{Text: x, Kind: segment.Word})
	}
	return t
}

func TestMarkerDetector(t *testing.T) {
	got := MarkerDetector{}.Detect(words("我", "把", "书", "看", "了"))
	want := []string{"le-aspect", "ba-construction"} // rule order: ba before le
	// rule order is ba, bei, le, ...; so expect ["ba-construction","le-aspect"]
	want = []string{"ba-construction", "le-aspect"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("detected %v, want %v", got, want)
	}
}

func TestMarkerDetectorEmpty(t *testing.T) {
	if got := (MarkerDetector{}).Detect(words("山", "水")); len(got) != 0 {
		t.Errorf("detected %v, want none", got)
	}
}
