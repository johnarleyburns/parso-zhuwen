// Command hskingest builds the real HSK-3.0 lexicon TSVs (factory/data/hsk3.0/level-*.tsv)
// from the official vocabulary lists, so `zhuwenctl lexicon ingest` can produce the real
// lexicon.sqlite (~11k stable word IDs, exact per-level mapping). It is a build-time data tool
// (like genfixtures) — never run in CI.
//
// Source: the official 《国际中文教育中文水平等级标准》 (HSK 3.0, MoE/CLEC, 2021) vocabulary,
// published per level at https://www.hanzistroke.com/hsk/3.0/level-N. The pages embed each
// entry as a React-Server-Components object `official_vocab:hsk30:<idx0>:<idx1>` carrying the
// word, pinyin, and part of speech; `idx1` is the entry's global 1-based position in the
// official ordering and is used directly as the **stable word ID** ("word IDs are forever").
//
// Licensing: the underlying word list is the government HSK-3.0 standard; redistribution as
// part of a language-learning app has been confirmed by the project owner (see
// factory/data/README.md and plans/blockers.md B-1). Only the derived (word, pinyin, level)
// mapping is emitted — no site markup or copyrighted prose.
//
// Usage:
//
//	hskingest --fetch --out factory/data/hsk3.0            # download + build
//	hskingest --src <dir-of-hsk-lN.html> --out <outdir>    # build from local pages
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// level is one HSK-3.0 level page. hsk is the value written to the lexicon's hsk3_level column;
// the advanced band 7–9 is not split lexically in the published list, so it maps to level 7.
type level struct {
	hsk  int
	file string // local filename (hsk-l<tag>.html)
	url  string
	tag  string // filename tag used for the output TSV (level-<tag>.tsv)
}

var levels = []level{
	{1, "hsk-l1.html", "https://www.hanzistroke.com/hsk/3.0/level-1", "1"},
	{2, "hsk-l2.html", "https://www.hanzistroke.com/hsk/3.0/level-2", "2"},
	{3, "hsk-l3.html", "https://www.hanzistroke.com/hsk/3.0/level-3", "3"},
	{4, "hsk-l4.html", "https://www.hanzistroke.com/hsk/3.0/level-4", "4"},
	{5, "hsk-l5.html", "https://www.hanzistroke.com/hsk/3.0/level-5", "5"},
	{6, "hsk-l6.html", "https://www.hanzistroke.com/hsk/3.0/level-6", "6"},
	{7, "hsk-l7-9.html", "https://www.hanzistroke.com/hsk/3.0/level-7-9", "7-9"},
}

// entry is one extracted vocabulary item.
type entry struct {
	id     int // official global 1-based index (stable word ID)
	simp   string
	pinyin string
	hsk    int
}

// objRe captures (idx0, idx1, form, pinyin) from an embedded official_vocab object (the
// HSK-3.0 vocabulary dimension). Single characters carry a "char" field, multi-character words a
// "simplified" field. The pinyin field may not be adjacent (a "traditional"/"definition" field
// can intervene), so we scan to it within the object bounds (`[^}]*?`).
var objRe = regexp.MustCompile(`\\"id\\":\\"official_vocab:hsk30:(\d+):(\d+)\\",\\"(?:char|simplified)\\":\\"([^\\"]+)\\"[^}]*?\\"pinyin\\":\\"([^\\"]*)\\"`)

// recogRe captures the recognition-character dimension (认读字): objects whose id IS the
// character itself (e.g. {"id":"里","char":"里","pinyin":"lǐ",...}). These single characters are
// part of the official standard but are not always listed as standalone vocabulary, so they must
// be added for complete text coverage. Group 1=id, 2=char, 3=pinyin; we keep those where id==char.
var recogRe = regexp.MustCompile(`\\"id\\":\\"([^\\"]+)\\",\\"char\\":\\"([^\\"]+)\\"[^}]*?\\"pinyin\\":\\"([^\\"]*)\\"`)

// recogBase is the ID range for recognition-only characters (kept disjoint from the official
// vocabulary global index, which tops out near 11000). IDs are assigned deterministically.
const recogBase = 20000

func main() {
	fetch := flag.Bool("fetch", false, "download the level pages before parsing")
	src := flag.String("src", ".", "directory containing hsk-l<tag>.html pages")
	out := flag.String("out", "factory/data/hsk3.0", "output directory for level-*.tsv")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		die(err)
	}

	// Parse every level, tagging each entry with its level; a homograph (same written form at
	// several readings/levels) is resolved to its lowest-id (earliest/lowest-level) occurrence.
	best := map[string]entry{}
	perLevelRaw := map[int]int{}
	recogSeq := 0
	for _, lv := range levels {
		path := filepath.Join(*src, lv.file)
		if *fetch {
			if err := download(lv.url, path); err != nil {
				die(fmt.Errorf("fetch %s: %w", lv.url, err))
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			die(err)
		}
		html := string(data)

		// Dimension 1: official vocabulary (words + vocab characters), stable global-index IDs.
		matches := objRe.FindAllStringSubmatch(html, -1)
		perLevelRaw[lv.hsk] += len(matches)
		for _, m := range matches {
			id, _ := strconv.Atoi(m[2])
			addEntry(best, entry{id: id, simp: m[3], pinyin: m[4], hsk: lv.hsk})
		}

		// Dimension 2: recognition characters (id == char). Assigned disjoint synthetic IDs in
		// source order so they are stable across runs.
		for _, m := range recogRe.FindAllStringSubmatch(html, -1) {
			id, char, pinyin := m[1], m[2], m[3]
			if id != char || strings.Contains(id, "official_vocab") {
				continue // only the recognition-character shape (id IS the character)
			}
			if _, seen := best[char]; seen {
				continue // already covered by the vocabulary dimension (lower/equal level)
			}
			recogSeq++
			addEntry(best, entry{id: recogBase + recogSeq, simp: char, pinyin: pinyin, hsk: lv.hsk})
		}
	}

	// Group deduped entries by level, sorted by stable id.
	byLevel := map[int][]entry{}
	for _, e := range best {
		byLevel[e.hsk] = append(byLevel[e.hsk], e)
	}
	total := 0
	fmt.Println("HSK 3.0 lexicon build:")
	for _, lv := range levels {
		es := byLevel[lv.hsk]
		sort.Slice(es, func(i, j int) bool { return es[i].id < es[j].id })
		if err := writeTSV(filepath.Join(*out, "level-"+lv.tag+".tsv"), lv, es); err != nil {
			die(err)
		}
		fmt.Printf("  level %-3s: %5d words (raw %d)\n", lv.tag, len(es), perLevelRaw[lv.hsk])
		total += len(es)
	}
	fmt.Printf("  total   : %5d unique words (of %d raw entries; homographs deduped to lowest level)\n",
		total, sumRaw(perLevelRaw))
}

func writeTSV(path string, lv level, es []entry) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# HSK 3.0 %s — official vocabulary (word IDs = official global index; stable forever)\n", "level-"+lv.tag)
	fmt.Fprintf(&b, "# columns: id\\tsimp\\tpinyin\\thsk3_level\\tfreq_rank\n")
	fmt.Fprintf(&b, "# source: https://www.hanzistroke.com/hsk/3.0/level-%s · retrieved %s\n", lv.tag, time.Now().UTC().Format("2006-01-02"))
	for _, e := range es {
		// freq_rank uses the official global ordering (≈ level/frequency order); see data README.
		fmt.Fprintf(&b, "%d\t%s\t%s\t%d\t%d\n", e.id, e.simp, e.pinyin, e.hsk, e.id)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func download(url, dst string) error {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (zhuwen hskingest)")
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, body, 0o644)
}

func sumRaw(m map[int]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

// addEntry inserts e into best if it is the lowest-level occurrence of its written form (or, at
// the same level, the lowest id). Keeps homographs collapsed to one stable entry per form.
func addEntry(best map[string]entry, e entry) {
	cur, ok := best[e.simp]
	if !ok || e.hsk < cur.hsk || (e.hsk == cur.hsk && e.id < cur.id) {
		best[e.simp] = e
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "hskingest:", err)
	os.Exit(1)
}
