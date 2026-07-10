package images

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const commonsURL = "https://commons.wikimedia.org/w/api.php"

// DefaultUserAgent is the User-Agent string sent to Wikimedia Commons.
const DefaultUserAgent = "Zhuwen-Factory/0.1 (https://github.com/johnarleyburns/parso-zhuwen; language-learning app)"

// FetchClient queries the Wikimedia Commons API for image candidates. The zero value
// uses http.DefaultClient and DefaultUserAgent; set HTTPClient for testing.
type FetchClient struct {
	HTTPClient *http.Client
	UserAgent  string
}

func (fc *FetchClient) client() *http.Client {
	if fc.HTTPClient != nil {
		return fc.HTTPClient
	}
	return http.DefaultClient
}

func (fc *FetchClient) ua() string {
	if fc.UserAgent != "" {
		return fc.UserAgent
	}
	return DefaultUserAgent
}

// FetchCandidate queries Commons for a search term, returning up to n candidates parsed
// from the API response.
func (fc *FetchClient) FetchCandidate(term string, n int) ([]Candidate, error) {
	q := url.Values{
		"action":       {"query"},
		"format":       {"json"},
		"generator":    {"search"},
		"gsrsearch":    {"filetype:bitmap " + term},
		"gsrnamespace": {"6"},
		"gsrlimit":     {fmt.Sprint(n)},
		"prop":         {"imageinfo|categories"},
		"iiprop":       {"extmetadata|url|size"},
		"iiurlwidth":   {"360"},
		"cllimit":      {"200"},
	}
	return fc.parsePages(commonsURL + "?" + q.Encode())
}

// FetchProvenanceByTitles queries Commons for a batch of specific File: titles (up to 50
// per API call) and returns their provenance (license/author/dimensions/source URL). This
// is the CP-09c curate path: given the reviewer's chosen files, fetch the attribution
// record that ships in the pack. Network; behind --live at the command layer (I2).
func (fc *FetchClient) FetchProvenanceByTitles(titles []string) (ProvenanceStore, error) {
	store := ProvenanceStore{}
	const batch = 50
	for start := 0; start < len(titles); start += batch {
		end := start + batch
		if end > len(titles) {
			end = len(titles)
		}
		chunk := titles[start:end]
		q := url.Values{
			"action": {"query"},
			"format": {"json"},
			"titles": {strings.Join(chunk, "|")},
			"prop":   {"imageinfo"},
			"iiprop": {"extmetadata|url|size"},
		}
		req, err := http.NewRequest("GET", commonsURL+"?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", fc.ua())
		resp, err := fc.client().Do(req)
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		provs, err := parseProvenanceJSON(b)
		if err != nil {
			return nil, err
		}
		for title, p := range provs {
			store[title] = p
		}
		if end < len(titles) {
			time.Sleep(120 * time.Millisecond)
		}
	}
	return store, nil
}

// parseProvenanceJSON parses a titles=…&prop=imageinfo response into a ProvenanceStore
// keyed by the normalized File: title. Commons may "normalize" titles (spaces/underscores);
// the normalization map is applied so lookups by the requested title still resolve.
func parseProvenanceJSON(b []byte) (ProvenanceStore, error) {
	var r struct {
		Query struct {
			Normalized []struct {
				From string `json:"from"`
				To   string `json:"to"`
			} `json:"normalized"`
			Pages map[string]struct {
				Title     string `json:"title"`
				Missing   *bool  `json:"missing"`
				ImageInfo []struct {
					DescURL     string             `json:"descriptionurl"`
					Width       int                `json:"width"`
					Height      int                `json:"height"`
					ExtMetadata map[string]extMeta `json:"extmetadata"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("parse Commons provenance: %w", err)
	}
	store := ProvenanceStore{}
	for _, p := range r.Query.Pages {
		if p.Missing != nil || len(p.ImageInfo) == 0 {
			continue
		}
		ii := p.ImageInfo[0]
		licURL := ii.ExtMetadata["LicenseUrl"].String()
		if licURL == "" {
			licURL = commonsFileURL(p.Title)
		}
		src := ii.DescURL
		if src == "" {
			src = commonsFileURL(p.Title)
		}
		author := clean(ii.ExtMetadata["Artist"].String())
		license := clean(ii.ExtMetadata["LicenseShortName"].String())
		if author == "" {
			// PD/CC0 works (e.g. museum scans) often carry no Artist; attribution is not
			// legally required, but I6 requires a non-empty author field. Use the credit
			// line if present, else a defensible public-domain author sentinel.
			if credit := clean(ii.ExtMetadata["Credit"].String()); credit != "" {
				author = credit
			} else if isPublicDomainLicense(license) {
				author = "Unknown (public domain)"
			}
		}
		store[p.Title] = Provenance{
			File:        p.Title,
			License:     license,
			LicenseURL:  clean(licURL),
			Author:      author,
			SourceURL:   src,
			RetrievedAt: time.Now().UTC().Format("2006-01-02"),
			W:           ii.Width,
			H:           ii.Height,
		}
	}
	// Map requested (pre-normalization) titles to their normalized provenance too.
	for _, n := range r.Query.Normalized {
		if p, ok := store[n.To]; ok {
			store[n.From] = p
		}
	}
	return store, nil
}

// FetchCandidates queries Commons using both English and Chinese terms for a word,
// gathering candidates. English results are preferred; if fewer than minEn results
// come from the English query, the Chinese term is also searched.
func (fc *FetchClient) FetchCandidates(en, zh string, n int, minEn int) ([]Candidate, error) {
	seen := map[string]bool{}
	var order []Candidate

	enCands, err := fc.FetchCandidate(en, n)
	if err != nil {
		return nil, fmt.Errorf("fetch en: %w", err)
	}
	for _, c := range enCands {
		if c.Title == "" {
			continue
		}
		if seen[c.Title] {
			continue
		}
		seen[c.Title] = true
		order = append(order, c)
	}

	if len(order) < minEn && zh != "" {
		time.Sleep(120 * time.Millisecond)
		zhCands, err := fc.FetchCandidate(zh, n)
		if err != nil {
			return nil, fmt.Errorf("fetch zh: %w", err)
		}
		for _, c := range zhCands {
			if c.Title == "" {
				continue
			}
			if seen[c.Title] {
				continue
			}
			seen[c.Title] = true
			order = append(order, c)
		}
	}
	return order, nil
}

func (fc *FetchClient) parsePages(u string) ([]Candidate, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fc.ua())
	resp, err := fc.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parsePagesJSON(b)
}

func parsePagesJSON(b []byte) ([]Candidate, error) {
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
					ExtMetadata map[string]extMeta `json:"extmetadata"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("parse Commons response: %w", err)
	}
	var out []Candidate
	for _, p := range r.Query.Pages {
		if len(p.ImageInfo) == 0 {
			continue
		}
		ii := p.ImageInfo[0]
		lic := ii.ExtMetadata["LicenseShortName"].String()
		licURL := ii.ExtMetadata["LicenseUrl"].String()
		if licURL == "" {
			licURL = commonsFileURL(p.Title)
		}
		out = append(out, Candidate{
			Title:      p.Title,
			ThumbURL:   ii.ThumbURL,
			DescURL:    ii.DescURL,
			W:          ii.Width,
			H:          ii.Height,
			License:    clean(lic),
			LicenseURL: clean(licURL),
			Author:     clean(ii.ExtMetadata["Artist"].String()),
			Categories: categoryTitles(p.Categories),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		ii := findPageIndex(r.Query.Pages, out[i].Title)
		jj := findPageIndex(r.Query.Pages, out[j].Title)
		return ii < jj
	})
	return out, nil
}

func findPageIndex(pages map[string]struct {
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
		ExtMetadata map[string]extMeta `json:"extmetadata"`
	} `json:"imageinfo"`
}, title string) int {
	for _, p := range pages {
		if p.Title == title {
			return p.Index
		}
	}
	return 999999
}

func categoryTitles(cats []struct {
	Title string `json:"title"`
}) []string {
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		out = append(out, c.Title)
	}
	return out
}

func commonsFileURL(title string) string {
	return "https://commons.wikimedia.org/wiki/" + strings.ReplaceAll(title, " ", "_")
}

// isPublicDomainLicense reports whether a license short-name denotes public-domain / CC0,
// where author attribution is not legally required (so a PD sentinel author is acceptable).
func isPublicDomainLicense(lic string) bool {
	l := strings.ToLower(lic)
	return strings.Contains(l, "public domain") || strings.Contains(l, "cc0") ||
		strings.Contains(l, "pd") || strings.Contains(l, "zero")
}

// extMeta handles the extmetadata field which is sometimes a number value.
type extMeta struct{ Value any }

func (e extMeta) String() string {
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

var (
	tagStrip   = strings.NewReplacer("<", ">")
	wsCollapse = strings.NewReplacer // handled inline
)

func clean(s string) string {
	s = stripTags(s)
	return strings.Join(strings.Fields(s), " ")
}

func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}
