package lexicon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `# comment
1	山	shān	1	300
2	水	shuǐ	1	250
11	坚持	jiānchí	4	1600	3,4
`

func TestIngestAssignsStableIDsAndLookups(t *testing.T) {
	lex, err := Ingest(strings.NewReader(sample), "v0")
	if err != nil {
		t.Fatal(err)
	}
	if lex.Len() != 3 {
		t.Fatalf("len = %d, want 3", lex.Len())
	}
	if lex.Version() != "v0" {
		t.Errorf("version = %q", lex.Version())
	}
	if lex.MaxID() != 11 {
		t.Errorf("maxID = %d, want 11", lex.MaxID())
	}
	w, ok := lex.LookupSimp("坚持")
	if !ok || w.ID != 11 || w.HSK != 4 || w.FreqRank != 1600 {
		t.Fatalf("lookup 坚持 = %+v ok=%v", w, ok)
	}
	if len(w.CharIDs) != 2 || w.CharIDs[0] != 3 || w.CharIDs[1] != 4 {
		t.Errorf("char_ids = %v, want [3 4]", w.CharIDs)
	}
	byID, ok := lex.LookupID(1)
	if !ok || byID.Simp != "山" {
		t.Errorf("lookup id 1 = %+v ok=%v", byID, ok)
	}
	if _, ok := lex.LookupID(999); ok {
		t.Error("lookup of missing id should fail")
	}
}

func TestIngestDictEntries(t *testing.T) {
	lex, _ := Ingest(strings.NewReader(sample), "v0")
	d := lex.DictEntries()
	if d["山"] != 1 || d["坚持"] != 11 {
		t.Errorf("dict entries wrong: %v", d)
	}
}

func TestIngestRejectsDuplicatesAndBadRows(t *testing.T) {
	cases := map[string]string{
		"dup id":     "1\t山\ta\t1\t1\n1\t水\tb\t1\t2\n",
		"dup simp":   "1\t山\ta\t1\t1\n2\t山\tb\t1\t2\n",
		"bad id":     "0\t山\ta\t1\t1\n",
		"too few":    "1\t山\ta\t1\n",
		"bad hsk":    "1\t山\ta\tX\t1\n",
		"empty simp": "1\t\ta\t1\t1\n",
	}
	for name, in := range cases {
		if _, err := Ingest(strings.NewReader(in), "v0"); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestIngestFileFromDisk(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "lex.tsv")
	if err := os.WriteFile(p, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	lex, err := IngestFile(p, "disk")
	if err != nil {
		t.Fatal(err)
	}
	if lex.Len() != 3 {
		t.Fatalf("len = %d", lex.Len())
	}
}

func TestIngestEmptyFails(t *testing.T) {
	if _, err := Ingest(strings.NewReader("# only comments\n"), "v0"); err == nil {
		t.Error("expected error for empty lexicon")
	}
}
