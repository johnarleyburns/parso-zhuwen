// Package segment implements the factory segmenter (handoff §4.3). The coverage gate
// operates ONLY on this factory segmentation (risk table: segmentation disagreements).
// CP-01 uses forward-maximum-matching (FMM) over the frozen lexicon dictionary plus a
// per-story proper-noun dictionary. jieba/pkuseg compatibility is a CP-09 concern; the
// interface (text -> tagged tokens) is stable.
package segment

// Kind classifies a token.
type Kind int

const (
	// Word is an in-lexicon word (has WordID).
	Word Kind = iota
	// Literal is an out-of-lexicon run of one character (no WordID).
	Literal
	// ProperNoun is a declared named entity, excluded from the coverage denominator.
	ProperNoun
)

// Token is one segmented unit.
type Token struct {
	Text        string
	WordID      int // -1 for Literal / ProperNoun
	Kind        Kind
	SentenceIdx int
	Gloss       string // proper-noun gloss ("" if none supplied)
	First       bool   // first occurrence of this proper noun in the text
}

// Segmenter segments text using a frozen dictionary and proper-noun set.
type Segmenter struct {
	dict    map[string]int    // simp -> word id
	propers map[string]string // proper-noun text -> gloss
	maxLen  int               // longest key in runes
}

// New builds a segmenter from a lexicon dictionary (simp->id) and a proper-noun
// gloss map (text->gloss; gloss may be "").
func New(dict map[string]int, propers map[string]string) *Segmenter {
	s := &Segmenter{dict: dict, propers: propers}
	if s.propers == nil {
		s.propers = map[string]string{}
	}
	for k := range dict {
		if n := len([]rune(k)); n > s.maxLen {
			s.maxLen = n
		}
	}
	for k := range s.propers {
		if n := len([]rune(k)); n > s.maxLen {
			s.maxLen = n
		}
	}
	if s.maxLen == 0 {
		s.maxLen = 1
	}
	return s
}

// sentence terminators split the token stream into sentences.
var terminators = map[rune]bool{'。': true, '！': true, '？': true, '!': true, '?': true}

// skippable characters produce no token (punctuation, whitespace).
func skippable(r rune) bool {
	switch r {
	case '，', '、', '；', '：', '“', '”', '‘', '’', '（', '）', '《', '》',
		'…', '—', ' ', '\t', '\n', ',', ';', ':', '(', ')', '"', '\'':
		return true
	}
	return false
}

// Segment tokenizes text with FMM. Proper nouns win ties against dictionary words.
func (s *Segmenter) Segment(text string) []Token {
	runes := []rune(text)
	var out []Token
	seenProper := map[string]bool{}
	sent := 0
	i := 0
	for i < len(runes) {
		r := runes[i]
		if terminators[r] {
			sent++
			i++
			continue
		}
		if skippable(r) {
			i++
			continue
		}
		matched := false
		maxL := s.maxLen
		if maxL > len(runes)-i {
			maxL = len(runes) - i
		}
		for l := maxL; l >= 1; l-- {
			cand := string(runes[i : i+l])
			if gloss, ok := s.propers[cand]; ok {
				first := !seenProper[cand]
				seenProper[cand] = true
				out = append(out, Token{Text: cand, WordID: -1, Kind: ProperNoun, SentenceIdx: sent, Gloss: gloss, First: first})
				i += l
				matched = true
				break
			}
			if id, ok := s.dict[cand]; ok {
				out = append(out, Token{Text: cand, WordID: id, Kind: Word, SentenceIdx: sent})
				i += l
				matched = true
				break
			}
		}
		if !matched {
			out = append(out, Token{Text: string(r), WordID: -1, Kind: Literal, SentenceIdx: sent})
			i++
		}
	}
	return out
}
