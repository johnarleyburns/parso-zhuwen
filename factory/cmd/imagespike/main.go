// Command imagespike is the CP-08a §8A curation spike: it queries Wikimedia Commons
// for the Foundations F0 seed words, applies the license/AI/resolution gate,
// auto-picks a best-of-N "best guess" per word, and writes a self-contained HTML review
// sheet (no server, no external JS) plus picks.json. This is the human-review artifact for
// blocker B-4: open the HTML, approve/override each pick, and export a decisions JSON.
//
// Network is used only when run manually (Commons is anonymous, no secret); it is never
// invoked by `go test` / CI (I2 — this is a factory build-time tool, not the app).
// The productionized fetch/gate/curate/process stages land as internal/images per
// plans/cp-08a-plan.md; this command is that pipeline's spike seed.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const userAgent = "Zhuwen-Factory-imagespike/0.1 (https://github.com/johnarleyburns/parso-zhuwen; language-learning app)"

// F0 seed words (00 §5A.1). The WD field is a Wikidata label for P18 lookup (unused in
// this spike — Commons file search suffices — but kept so the field can age into
// internal/images).
var f0Seeds = []Seed{
	{Simp: "水", Pinyin: "shuǐ", En: "water", WD: "water", Set: "seeds"},
	{Simp: "狗", Pinyin: "gǒu", En: "dog", WD: "dog", Set: "seeds"},
	{Simp: "猫", Pinyin: "māo", En: "cat", WD: "cat", Set: "seeds"},
	{Simp: "宝宝", Pinyin: "bǎobao", En: "baby", WD: "infant", Set: "seeds"},
	{Simp: "人", Pinyin: "rén", En: "person", WD: "person", Set: "seeds"},
	{Simp: "山", Pinyin: "shān", En: "mountain", WD: "mountain", Set: "seeds"},
	{Simp: "火", Pinyin: "huǒ", En: "fire", WD: "fire", Set: "seeds"},
	{Simp: "鱼", Pinyin: "yú", En: "fish", WD: "fish", Set: "seeds"},
	{Simp: "米饭", Pinyin: "mǐfàn", En: "cooked rice", WD: "cooked rice", Set: "seeds"},
	{Simp: "茶", Pinyin: "chá", En: "tea", WD: "tea", Set: "seeds"},
	{Simp: "车", Pinyin: "chē", En: "car", WD: "car", Set: "seeds"},
}

type Seed struct {
	Simp, Pinyin, En, WD, Set string
	Abstract                  bool   // hard-to-photograph term: fetch more candidates for review
	Desc                      string // human-readable context (story title + plot) shown in the sheet
}

type Candidate struct {
	rank       int
	Title      string   `json:"title"`
	Thumb      string   `json:"thumb"`
	DescURL    string   `json:"source_url"`
	License    string   `json:"license"`
	LicenseURL string   `json:"license_url"`
	Author     string   `json:"author"`
	W          int      `json:"w"`
	H          int      `json:"h"`
	Categories []string `json:"-"`
	P18        bool     `json:"p18"`
	Score      float64  `json:"score"`
	Accepted   bool     `json:"accepted"`
	RejectWhy  string   `json:"reject_why,omitempty"`
}

type WordResult struct {
	Seed
	Best    *Candidate   `json:"best"`
	Alts    []*Candidate `json:"alternates"`
	Rejects []*Candidate `json:"rejected"`
}

// SetGroup buckets word results by semantic set for the grouped review sheet.
type SetGroup struct {
	Set   string
	Words []WordResult
}

func main() {
	out := flag.String("out", "/tmp/opencode/zhuwen-image-review", "output dir for review sheet + picks.json")
	n := flag.Int("n", 10, "candidates per word (pick-of-N)")
	nAbstract := flag.Int("n-abstract", 20, "candidates for words flagged abstract in the inventory (5th column = 1/true/abstract)")
	inv := flag.String("inventory", "", "TSV inventory (simp\\tpinyin\\ten\\tset[\\tabstract]); default = built-in F0 seeds")
	decided := flag.String("decided", "", "comma-separated decisions JSON file(s); words already decided are skipped")
	render := flag.String("render", "", "render HTML from an existing picks.json (no network) and exit")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}

	// Render-only: rebuild the review sheet from a saved picks.json (fast, offline).
	if *render != "" {
		b, err := os.ReadFile(*render)
		if err != nil {
			fail(err)
		}
		var results []WordResult
		if err := json.Unmarshal(b, &results); err != nil {
			fail(err)
		}
		html := filepath.Join(*out, "review.html")
		writeHTML(html, groupBySet(results))
		fmt.Printf("rendered %d words → %s\n\nOpen it:  open %q\n", len(results), html, html)
		return
	}

	seeds := f0Seeds
	if *inv != "" {
		var err error
		seeds, err = loadInventory(*inv)
		if err != nil {
			fail(err)
		}
		fmt.Fprintf(os.Stderr, "loaded %d words from %s\n", len(seeds), *inv)
	}

	if *decided != "" {
		done, err := loadDecided(strings.Split(*decided, ","))
		if err != nil {
			fail(err)
		}
		kept := seeds[:0:0]
		var skipped int
		for _, s := range seeds {
			if done[s.Simp] {
				skipped++
				continue
			}
			kept = append(kept, s)
		}
		seeds = kept
		fmt.Fprintf(os.Stderr, "skipped %d already-decided words; %d remain to review\n", skipped, len(seeds))
	}

	var results []WordResult
	for i, s := range seeds {
		want := *n
		if s.Abstract {
			want = *nAbstract
		}
		tag := ""
		if s.Abstract {
			tag = " [abstract]"
		}
		fmt.Fprintf(os.Stderr, "· [%d/%d] %s (%s)%s…\n", i+1, len(seeds), s.Simp, s.En, tag)
		cands := gather(s, want)
		for j := range cands {
			gate(cands[j])
		}
		wr := WordResult{Seed: s}
		for _, c := range cands {
			switch {
			case !c.Accepted:
				wr.Rejects = append(wr.Rejects, c)
			case wr.Best == nil:
				wr.Best = c
			default:
				wr.Alts = append(wr.Alts, c)
			}
		}
		results = append(results, wr)
		time.Sleep(120 * time.Millisecond)
	}

	// Group by set, preserving first-seen set order.
	groups := groupBySet(results)

	picks := filepath.Join(*out, "picks.json")
	writeJSON(picks, results)
	html := filepath.Join(*out, "review.html")
	writeHTML(html, groups)
	var picked int
	for _, wr := range results {
		if wr.Best != nil {
			picked++
		}
	}
	fmt.Printf("\n%d/%d words have a gate-passing pick\nReview sheet: %s\nPicks JSON:   %s\n\nOpen it:  open %q\n",
		picked, len(results), html, picks, html)
}

// groupBySet buckets results into semantic sets, preserving first-seen set order.
func groupBySet(results []WordResult) []SetGroup {
	var groups []SetGroup
	idx := map[string]int{}
	for _, wr := range results {
		set := wr.Set
		if set == "" {
			set = "seeds"
		}
		if _, ok := idx[set]; !ok {
			idx[set] = len(groups)
			groups = append(groups, SetGroup{Set: set})
		}
		groups[idx[set]].Words = append(groups[idx[set]].Words, wr)
	}
	return groups
}

// loadDecided reads decisions JSON file(s) ([{word|simp, decision}]) and returns the set of
// words that already have a non-empty, non-reject decision — those are skipped on re-run.
// Files may key the word as either "word" or "simp" (the two formats in-repo); both are
// matched against the inventory's Simp field so either export skips correctly.
func loadDecided(paths []string) (map[string]bool, error) {
	done := map[string]bool{}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var recs []struct {
			Word     string `json:"word"`
			Simp     string `json:"simp"`
			Decision string `json:"decision"`
		}
		if err := json.Unmarshal(b, &recs); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		for _, r := range recs {
			key := r.Simp
			if key == "" {
				key = r.Word
			}
			if key != "" && r.Decision != "" && r.Decision != "__reject__" {
				done[key] = true
			}
		}
	}
	return done, nil
}

// loadInventory reads a TSV: simp<TAB>pinyin<TAB>en<TAB>set[<TAB>abstract[<TAB>desc]].
// '#' lines ignored. The optional 5th column marks a hard-to-photograph word
// (1/true/abstract/y) so more candidates are fetched; the optional 6th column is a
// human-readable description (e.g. story title + plot) shown in the review sheet so the
// reviewer knows what the image is *for*.
func loadInventory(path string) ([]Seed, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var seeds []Seed
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 3 {
			continue
		}
		s := Seed{Simp: f[0], Pinyin: f[1], En: f[2]}
		if len(f) > 3 {
			s.Set = f[3]
		}
		if len(f) > 4 {
			switch strings.ToLower(strings.TrimSpace(f[4])) {
			case "1", "true", "abstract", "y", "yes":
				s.Abstract = true
			}
		}
		if len(f) > 5 {
			s.Desc = strings.TrimSpace(f[5])
		}
		seeds = append(seeds, s)
	}
	return seeds, nil
}

// gather queries Commons for both the English search term AND the Chinese title directly,
// merging the results (deduped by title). Chinese-title search is always run — for Chinese
// legends/idioms Commons often holds strong, on-concept images filed under the Chinese name
// (e.g. 守株待兔, 嫦娥奔月), which an English-term search misses. Results from both queries
// appear in the review sheet so the reviewer can pick from either.
func gather(s Seed, n int) []*Candidate {
	seen := map[string]*Candidate{}
	var order []*Candidate
	add := func(c *Candidate) {
		if c == nil || c.Title == "" {
			return
		}
		if _, ok := seen[c.Title]; ok {
			return
		}
		seen[c.Title] = c
		order = append(order, c)
	}
	for _, c := range commonsSearch(s.En, n) {
		add(c)
	}
	// Always also search the Chinese title/word directly (dedup handles overlap).
	if s.Simp != "" && s.Simp != s.En {
		for _, c := range commonsSearch(s.Simp, n) {
			add(c)
		}
	}
	return order
}

var (
	licenseOK  = regexp.MustCompile(`(?i)^(public domain|pd|cc0|cc[ -]?by([ -]?sa)?([ -]?\d[\d.]*)?)`)
	licenseBad = regexp.MustCompile(`(?i)(\bnc\b|noncommercial|non-commercial|\bnd\b|noderiv|gfdl|fair use|all rights reserved)`)
	tagStrip   = regexp.MustCompile(`<[^>]*>`)
	wsCollapse = regexp.MustCompile(`\s+`)
)

// gate applies the §8A hard license gate, AI-category exclusion, and the 1200px floor,
// then scores accepted candidates for the best-of-N pick.
func gate(c *Candidate) {
	for _, cat := range c.Categories {
		if strings.Contains(strings.ToLower(cat), "ai-generated") {
			c.Accepted, c.RejectWhy = false, "AI-generated category (I6)"
			return
		}
	}
	lic := strings.TrimSpace(c.License)
	if lic == "" {
		c.Accepted, c.RejectWhy = false, "missing/ambiguous license"
		return
	}
	if licenseBad.MatchString(lic) || !licenseOK.MatchString(lic) {
		c.Accepted, c.RejectWhy = false, "license not PD/CC0/CC-BY/CC-BY-SA: "+lic
		return
	}
	if min(c.W, c.H) < 1200 {
		c.Accepted, c.RejectWhy = false, fmt.Sprintf("short side %dpx < 1200", min(c.W, c.H))
		return
	}
	c.Accepted = true
	// Score: prefer PD/CC0 > CC-BY > CC-BY-SA; prefer higher resolution.
	score := float64(min(c.W, c.H)) / 1000.0
	switch {
	case regexp.MustCompile(`(?i)(public domain|pd|cc0)`).MatchString(lic):
		score += 30
	case regexp.MustCompile(`(?i)cc[ -]?by[ -]?sa`).MatchString(lic):
		score += 10
	default:
		score += 20 // CC-BY (no SA)
	}
	c.Score = score
}

// --- Wikimedia API ---------------------------------------------------------

// ExtMeta handles the inconsistent extmetadata field which is sometimes a number
// (e.g. "DateTime": 0 when the Commons API returns numeric zero).
type ExtMeta struct {
	Value any `json:"value"`
}

func (e ExtMeta) String() string {
	if e.Value == nil {
		return ""
	}
	switch v := e.Value.(type) {
	case string:
		return v
	case float64:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func commonsSearch(term string, n int) []*Candidate {
	q := url.Values{
		"action": {"query"}, "format": {"json"},
		"generator": {"search"},
		"gsrsearch": {"filetype:bitmap " + term}, "gsrnamespace": {"6"}, "gsrlimit": {fmt.Sprint(n)},
		"prop":   {"imageinfo|categories"},
		"iiprop": {"extmetadata|url|size"}, "iiurlwidth": {"360"},
		"cllimit": {"200"},
	}
	return parsePages("https://commons.wikimedia.org/w/api.php?" + q.Encode())
}

func parsePages(u string) []*Candidate {
	var r struct {
		Query struct {
			Pages map[string]struct {
				Title      string `json:"title"`
				Index      int    `json:"index"`
				Categories []struct {
					Title string `json:"title"`
				} `json:"categories"`
				ImageInfo []struct {
					ThumbURL    string             `json:"thumburl"`
					DescURL     string             `json:"descriptionurl"`
					Width       int                `json:"width"`
					Height      int                `json:"height"`
					ExtMetadata map[string]ExtMeta `json:"extmetadata"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if !getJSON(u, &r) {
		return nil
	}
	var out []*Candidate
	for _, p := range r.Query.Pages {
		if len(p.ImageInfo) == 0 {
			continue
		}
		ii := p.ImageInfo[0]
		c := &Candidate{
			rank:       p.Index,
			Title:      p.Title,
			Thumb:      ii.ThumbURL,
			DescURL:    ii.DescURL,
			W:          ii.Width,
			H:          ii.Height,
			License:    clean(ii.ExtMetadata["LicenseShortName"].String()),
			LicenseURL: clean(ii.ExtMetadata["LicenseUrl"].String()),
			Author:     clean(ii.ExtMetadata["Artist"].String()),
		}
		for _, cat := range p.Categories {
			c.Categories = append(c.Categories, cat.Title)
		}
		out = append(out, c)
	}
	// Commons returns pages as a map (random order); the search generator's "index"
	// field carries relevance rank — sort by it so gather order == relevance order.
	sort.SliceStable(out, func(i, j int) bool { return out[i].rank < out[j].rank })
	return out
}

func getJSON(u string, v any) bool {
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "  http:", err)
		return false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(b, v); err != nil {
		fmt.Fprintln(os.Stderr, "  json:", err)
		return false
	}
	return true
}

func clean(s string) string {
	return strings.TrimSpace(wsCollapse.ReplaceAllString(tagStrip.ReplaceAllString(s, " "), " "))
}

func writeJSON(path string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		fail(err)
	}
}

func writeHTML(path string, groups []SetGroup) {
	f, err := os.Create(path)
	if err != nil {
		fail(err)
	}
	defer f.Close()
	if err := reviewTmpl.Execute(f, groups); err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, "imagespike:", err); os.Exit(1) }

var reviewTmpl = template.Must(template.New("review").Funcs(template.FuncMap{
	"px": func(c *Candidate) string { return fmt.Sprintf("%d×%d", c.W, c.H) },
}).Parse(reviewHTML))
