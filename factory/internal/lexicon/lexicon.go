// Package lexicon ingests the HSK-3.0 word list into a read-only lexicon with
// stable integer word IDs (handoff §4.1: "Word IDs are forever"; levels/ranks are
// attributes). CP-01 ingests a TSV fixture; the production ingest reads the official
// lists but produces the same shape.
package lexicon

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Word is one lexicon entry. ID is stable across pack versions.
type Word struct {
	ID       int
	Simp     string
	Pinyin   string
	HSK      int
	FreqRank int
	CharIDs  []int
}

// Lexicon is an immutable, read-only view over the ingested word list.
type Lexicon struct {
	version string
	words   []Word
	byID    map[int]*Word
	bySimp  map[string]*Word
	maxID   int
}

// Version returns the lexicon_version that pins the word IDs.
func (l *Lexicon) Version() string { return l.version }

// Len returns the number of words.
func (l *Lexicon) Len() int { return len(l.words) }

// MaxID returns the largest word ID (bitmap sizing).
func (l *Lexicon) MaxID() int { return l.maxID }

// Words returns all words in ingest order.
func (l *Lexicon) Words() []Word { return l.words }

// LookupID returns the word with the given ID.
func (l *Lexicon) LookupID(id int) (*Word, bool) {
	w, ok := l.byID[id]
	return w, ok
}

// LookupSimp returns the word with the given simplified form.
func (l *Lexicon) LookupSimp(simp string) (*Word, bool) {
	w, ok := l.bySimp[simp]
	return w, ok
}

// IngestFile ingests a lexicon TSV from disk.
func IngestFile(path, version string) (*Lexicon, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Ingest(f, version)
}

// Ingest parses a tab-separated lexicon.
//
// Columns: id, simp, pinyin, hsk, freq_rank, char_ids(optional, comma-separated).
// Lines beginning with '#' and blank lines are ignored. Word IDs must be unique and
// positive; simplified forms must be unique.
func Ingest(r io.Reader, version string) (*Lexicon, error) {
	l := &Lexicon{
		version: version,
		byID:    map[int]*Word{},
		bySimp:  map[string]*Word{},
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	seenID := map[int]bool{}
	seenSimp := map[string]bool{}
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimRight(sc.Text(), "\r")
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		cols := strings.Split(raw, "\t")
		if len(cols) < 5 {
			return nil, fmt.Errorf("lexicon line %d: expected >=5 columns, got %d", line, len(cols))
		}
		id, err := strconv.Atoi(strings.TrimSpace(cols[0]))
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("lexicon line %d: invalid id %q", line, cols[0])
		}
		simp := strings.TrimSpace(cols[1])
		if simp == "" {
			return nil, fmt.Errorf("lexicon line %d: empty simp", line)
		}
		hsk, err := strconv.Atoi(strings.TrimSpace(cols[3]))
		if err != nil {
			return nil, fmt.Errorf("lexicon line %d: invalid hsk %q", line, cols[3])
		}
		freq, err := strconv.Atoi(strings.TrimSpace(cols[4]))
		if err != nil {
			return nil, fmt.Errorf("lexicon line %d: invalid freq_rank %q", line, cols[4])
		}
		var charIDs []int
		if len(cols) >= 6 && strings.TrimSpace(cols[5]) != "" {
			for _, p := range strings.Split(cols[5], ",") {
				c, err := strconv.Atoi(strings.TrimSpace(p))
				if err != nil {
					return nil, fmt.Errorf("lexicon line %d: invalid char_id %q", line, p)
				}
				charIDs = append(charIDs, c)
			}
		}
		if seenID[id] {
			return nil, fmt.Errorf("lexicon line %d: duplicate id %d", line, id)
		}
		if seenSimp[simp] {
			return nil, fmt.Errorf("lexicon line %d: duplicate simp %q", line, simp)
		}
		seenID[id] = true
		seenSimp[simp] = true
		w := Word{ID: id, Simp: simp, Pinyin: strings.TrimSpace(cols[2]), HSK: hsk, FreqRank: freq, CharIDs: charIDs}
		l.words = append(l.words, w)
		if id > l.maxID {
			l.maxID = id
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(l.words) == 0 {
		return nil, fmt.Errorf("lexicon: no words ingested")
	}
	// Build indexes against the stable backing slice.
	for i := range l.words {
		w := &l.words[i]
		l.byID[w.ID] = w
		l.bySimp[w.Simp] = w
	}
	return l, nil
}

// DictEntries returns simp->ID for building the segmenter dictionary (§4.3).
func (l *Lexicon) DictEntries() map[string]int {
	m := make(map[string]int, len(l.words))
	for i := range l.words {
		m[l.words[i].Simp] = l.words[i].ID
	}
	return m
}

// FromWords builds a Lexicon from an in-memory word slice (e.g. read back from
// lexicon.sqlite). Word IDs and simplified forms must be unique and positive; the
// resulting Lexicon is immutable and indexed like Ingest's output.
func FromWords(version string, words []Word) (*Lexicon, error) {
	l := &Lexicon{
		version: version,
		byID:    map[int]*Word{},
		bySimp:  map[string]*Word{},
	}
	seenID := map[int]bool{}
	seenSimp := map[string]bool{}
	for _, w := range words {
		if w.ID <= 0 {
			return nil, fmt.Errorf("lexicon: invalid id %d for %q", w.ID, w.Simp)
		}
		if w.Simp == "" {
			return nil, fmt.Errorf("lexicon: empty simp for id %d", w.ID)
		}
		if seenID[w.ID] {
			return nil, fmt.Errorf("lexicon: duplicate id %d", w.ID)
		}
		if seenSimp[w.Simp] {
			return nil, fmt.Errorf("lexicon: duplicate simp %q", w.Simp)
		}
		seenID[w.ID] = true
		seenSimp[w.Simp] = true
		l.words = append(l.words, w)
		if w.ID > l.maxID {
			l.maxID = w.ID
		}
	}
	if len(l.words) == 0 {
		return nil, fmt.Errorf("lexicon: no words")
	}
	for i := range l.words {
		w := &l.words[i]
		l.byID[w.ID] = w
		l.bySimp[w.Simp] = w
	}
	return l, nil
}
