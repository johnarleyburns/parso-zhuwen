package lexicon

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReadSQLiteRoundTrip(t *testing.T) {
	src := "1\t山\tshān\t1\t10\t\n" +
		"2\t坚持\tjiānchí\t3\t900\t101,102\n" +
		"3\t猫\tmāo\t1\t50\t\n"
	lex, err := Ingest(strings.NewReader(src), "test-v1")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	path := filepath.Join(t.TempDir(), "lexicon.sqlite")
	if err := WriteSQLite(lex, path); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadSQLite(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if got.Version() != "test-v1" {
		t.Errorf("version = %q, want test-v1", got.Version())
	}
	if got.Len() != lex.Len() {
		t.Fatalf("len = %d, want %d", got.Len(), lex.Len())
	}
	if got.MaxID() != lex.MaxID() {
		t.Errorf("maxID = %d, want %d", got.MaxID(), lex.MaxID())
	}
	// Every word (id, simp, attrs, char_ids) must round-trip exactly.
	for _, w := range lex.Words() {
		g, ok := got.LookupID(w.ID)
		if !ok {
			t.Fatalf("id %d missing after round-trip", w.ID)
		}
		if g.Simp != w.Simp || g.Pinyin != w.Pinyin || g.HSK != w.HSK || g.FreqRank != w.FreqRank {
			t.Errorf("id %d: got %+v, want %+v", w.ID, *g, w)
		}
		if len(g.CharIDs) != len(w.CharIDs) {
			t.Errorf("id %d char_ids = %v, want %v", w.ID, g.CharIDs, w.CharIDs)
		}
	}
	// The frozen word 坚持 keeps id 2 with its char_ids — "word IDs are forever" (§4.1).
	w, ok := got.LookupSimp("坚持")
	if !ok || w.ID != 2 || len(w.CharIDs) != 2 {
		t.Errorf("坚持 = %+v (ok=%v), want id 2 with 2 char_ids", w, ok)
	}
}

func TestFromWordsRejectsDuplicates(t *testing.T) {
	_, err := FromWords("v", []Word{{ID: 1, Simp: "山"}, {ID: 1, Simp: "水"}})
	if err == nil {
		t.Fatal("expected duplicate id error")
	}
}
