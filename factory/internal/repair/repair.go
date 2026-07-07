// Package repair implements the iterative retelling-repair loop (§4.4). When a gate Result
// fails, the failure codes (stable, machine-readable) drive a targeted rewrite prompt that
// names the exact violations. The repairer retries up to MaxIterations times, then discards
// the brief and logs its fate. The prompts are pure text generation; CI tests them
// hermetically by comparing built prompts to golden strings. No invariants are weakened: the
// gate budget (I1) is locked, and the repair loop only changes the input text, never the gate
// itself.
package repair

import (
	"fmt"
	"sort"
	"strings"

	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/gen"
	"github.com/parso/zhuwen-factory/internal/lexicon"
)

// MaxIterations is the cap on repair attempts before discarding (§4.4).
const MaxIterations = 4

// Fate records the outcome of a repair cycle (used in the spike report).
type Fate struct {
	Brief         brief.Brief
	Iterations    int
	Passed        bool
	Candidates    []gen.Story
	FailReasons   [][]string // per-iteration failure reasons
	FailCodes     [][]string // per-iteration stable failure codes (for the histogram)
	TotalTokens   int        // LLM token usage across all attempts for this story
	NewTokenTrace []int      // per-iteration NewTokens (over-budget count) — shows convergence
}

// Hint names one gate-violation fix request. These are derived from the stable failure
// machine codes in the gate package.
type Hint struct {
	Code, Desc string
}

// HintsFromResult maps the gate's machine codes to human-readable Cantonese/English repair
// instructions suitable for appending to a generation prompt.
func HintsFromResult(r gate.Result) []Hint {
	var out []Hint
	for _, code := range r.Codes {
		switch code {
		case gate.CodeTypeBudget:
			out = append(out, Hint{Code: code, Desc: "减少生词种类（当前超过 8 个词型）"})
		case gate.CodeTokenBudget:
			out = append(out, Hint{Code: code, Desc: "减少生词出现次数（当前超过 2% 令牌比例）"})
		case gate.CodeRecurrence:
			out = append(out, Hint{Code: code, Desc: "每个生词必须在正文中出现至少 3 次"})
		case gate.CodeFrontier:
			out = append(out, Hint{Code: code, Desc: "使用了不在允许新词列表中的生词——请只使用列表内的词"})
		case gate.CodeGrammar:
			out = append(out, Hint{Code: code, Desc: "使用了该语级不允许的语法结构——请用更简单的句子"})
		case gate.CodeLiteralOutOfLexicon:
			out = append(out, Hint{Code: code, Desc: "使用了词典中没有的汉字——请换用词典内或允许新词列表内的词"})
		case gate.CodeProperNounGloss:
			out = append(out, Hint{Code: code, Desc: "专有名词首次出现时必须加上解释（如“北京——中国的一个大城市”）"})
		}
	}
	return out
}

// RewritePrompt constructs the repair prompt — a system message listing every violation by
// code and reason, followed by the original brief contract. The LLM is expected to
// regenerate the text while fixing the listed issues. Pure function, golden-tested.
func RewritePrompt(res gate.Result, b brief.Brief) string {
	hints := HintsFromResult(res)
	if len(hints) == 0 {
		return "" // nothing to fix
	}
	var sb strings.Builder
	sb.WriteString("上一轮改写未通过词汇检查。请修正以下问题并重新输出中文正文（不要输出解释）：\n\n")
	sb.WriteString("失败原因：\n")
	for _, h := range hints {
		fmt.Fprintf(&sb, "· %s\n", h.Desc)
	}
	sb.WriteString("\n")
	sb.WriteString("失败的片段：")
	if len(res.Reasons) > 0 {
		for _, r := range res.Reasons {
			fmt.Fprintf(&sb, "  %s\n", r)
		}
	} else {
		sb.WriteString("（无更多信息）\n")
	}
	sb.WriteString("\n原文必须严格覆盖以下梗概：\n")
	for i, beat := range b.Beats {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, beat)
	}
	return sb.String()
}

// NameReplacePrompt is the token-level repair prompt (§4.4, MC-2 recommendation #2). Instead of
// naming counts/types it names each SPECIFIC offending word and supplies concrete in-band
// substitutes drawn from the known set, so the model performs targeted edits that actually pull
// the draft into the whitelist. Pure function of the gate result + lexicon; never loosens I1.
func NameReplacePrompt(res gate.Result, b brief.Brief, lex *lexicon.Lexicon, band gate.Band) string {
	if res.Pass || lex == nil {
		return ""
	}
	cfg := gate.DefaultConfig()
	simp := func(id int) string {
		if w, ok := lex.LookupID(id); ok {
			return w.Simp
		}
		return fmt.Sprintf("#%d", id)
	}

	var outOfFrontier, underRecurring []gate.NewTypeStat
	for _, nt := range res.NewTypeCounts {
		switch {
		case !nt.InFrontier:
			outOfFrontier = append(outOfFrontier, nt)
		case nt.Count < cfg.MinRecurrence:
			underRecurring = append(underRecurring, nt)
		}
	}

	subs := frequentKnown(lex, band, 8)
	subList := strings.Join(subs, "、")

	var sb strings.Builder
	sb.WriteString("上一稿未通过词汇门槛。请逐词按下面的指示修改，只输出修改后的中文正文（不要输出解释）。\n")
	sb.WriteString("保持情节不变，只替换或调整下列词语：\n\n")

	if len(outOfFrontier) > 0 {
		sb.WriteString("【必须替换的词】（超出允许范围，请改用已知词）：\n")
		for _, nt := range outOfFrontier {
			fmt.Fprintf(&sb, "· 「%s」→ 改用已知词（如：%s）\n", simp(nt.ID), subList)
		}
		sb.WriteString("\n")
	}

	if len(res.Literals) > 0 {
		sb.WriteString("【词典外的字】（词典中没有，请替换为已知词）：\n")
		for _, lit := range dedupe(res.Literals) {
			fmt.Fprintf(&sb, "· 「%s」→ 改用已知词（如：%s）\n", lit, subList)
		}
		sb.WriteString("\n")
	}

	if len(res.NewTypeCounts) > cfg.MaxNewTypes {
		sb.WriteString(fmt.Sprintf("【新词种类过多】：目前 %d 种，最多 %d 种。请删除或合并多余的新词，改用已知词。\n\n",
			len(res.NewTypeCounts), cfg.MaxNewTypes))
	}

	if len(underRecurring) > 0 {
		fmt.Fprintf(&sb, "【出现次数不足的新词】（每个新词必须出现至少 %d 次，否则请删除或改用已知词）：\n", cfg.MinRecurrence)
		for _, nt := range underRecurring {
			fmt.Fprintf(&sb, "· 「%s」（目前 %d 次）→ 增加到至少 %d 次，或替换为已知词\n", simp(nt.ID), nt.Count, cfg.MinRecurrence)
		}
		sb.WriteString("\n")
	}

	if allowed := frontierSimps(lex, band, 40); len(allowed) > 0 {
		fmt.Fprintf(&sb, "允许的新词只能从这些里选：%s\n\n", strings.Join(allowed, "、"))
	}

	if hasCode(res.Codes, gate.CodeGrammar) {
		sb.WriteString("【语法】：使用了该语级不允许的句式——请改用更简单的句子。\n\n")
	}

	if b.LengthMin > 0 {
		fmt.Fprintf(&sb, "请保持全文至少 %d 字（文章越长，生词比例越容易达标）；只替换词语，不要缩短情节。\n\n", b.LengthMin)
	}

	sb.WriteString("原文必须严格覆盖以下梗概：\n")
	for i, beat := range b.Beats {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, beat)
	}
	return sb.String()
}

// frequentKnown returns up to k of the most frequent known-set words (by freq rank), skipping
// grammar-marker characters. These are the in-band substitutes offered for out-of-band words.
func frequentKnown(lex *lexicon.Lexicon, band gate.Band, k int) []string {
	type wf struct {
		simp string
		rank int
	}
	ws := make([]wf, 0, len(band.Known))
	for id := range band.Known {
		w, ok := lex.LookupID(id)
		if !ok || isGrammarMarker(w.Simp) {
			continue
		}
		rank := w.FreqRank
		if rank <= 0 {
			rank = int(^uint(0) >> 1)
		}
		ws = append(ws, wf{w.Simp, rank})
	}
	sort.Slice(ws, func(i, j int) bool {
		if ws[i].rank != ws[j].rank {
			return ws[i].rank < ws[j].rank
		}
		return ws[i].simp < ws[j].simp
	})
	out := make([]string, 0, k)
	for _, w := range ws {
		out = append(out, w.simp)
		if len(out) >= k {
			break
		}
	}
	return out
}

// frontierSimps returns up to k frontier-candidate simps (sorted by ID) — the only words the
// model may introduce as new types.
func frontierSimps(lex *lexicon.Lexicon, band gate.Band, k int) []string {
	ids := make([]int, 0, len(band.Frontier))
	for id := range band.Frontier {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	out := make([]string, 0, k)
	for _, id := range ids {
		if w, ok := lex.LookupID(id); ok {
			out = append(out, w.Simp)
		}
		if len(out) >= k {
			break
		}
	}
	return out
}

func isGrammarMarker(s string) bool {
	switch s {
	case "把", "被", "了", "过", "的", "吗", "不", "在":
		return true
	}
	return false
}

func hasCode(codes []string, want string) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// Reprocessor orchestrates the retell → gate → repair loop for one brief. It is bound to a
// provider (real LLM or fixture) and a segmenter + band so the loop can run isolated.
type Reprocessor struct {
	Provider gen.Provider
	// Lex and Band enable token-level NAME-AND-REPLACE repair (§4.4, MC-2 rec #2): when Lex is
	// set the loop names each specific out-of-band word / under-recurring type and supplies an
	// in-band substitute drawn from the known set. When Lex is nil it falls back to the older
	// count/type RewritePrompt (keeps the loop testable with scripted providers).
	Lex  *lexicon.Lexicon
	Band gate.Band
}

// NewReprocessor builds a repair orchestrator over the given provider.
func NewReprocessor(p gen.Provider) *Reprocessor {
	return &Reprocessor{Provider: p}
}

// Run iterates: retell → produce token-level name-and-replace hints → retell with hints — up
// to MaxIterations. It records convergence stats (per-iteration over-budget token trace) and
// the per-story LLM token spend. Non-convergence discards the brief; the gate (I1) is never
// loosened — the loop only rewrites the input text.
func (r *Reprocessor) Run(b brief.Brief, checker gateChecker) Fate {
	var f Fate
	f.Brief = b
	f.Candidates = make([]gen.Story, 0, MaxIterations+1)
	f.FailReasons = make([][]string, MaxIterations+1)
	f.FailCodes = make([][]string, MaxIterations+1)
	startTokens := tokensOf(r.Provider)
	var prior gen.Story
	var lastResult gate.Result

	for iter := 0; iter <= MaxIterations; iter++ {
		var story gen.Story
		var err error
		if iter == 0 || prior.Text == "" {
			story, err = r.Provider.Retell(b)
		} else if rp, ok := r.Provider.(gen.RepairProvider); ok {
			// Feed the prior candidate + a targeted rewrite prompt so the model actually fixes
			// the named violations instead of regenerating blindly (§4.4).
			story, err = rp.RetellRepair(b, prior.Text, r.repairPrompt(lastResult, b))
		} else {
			story, err = r.Provider.Retell(b)
		}
		if err != nil {
			// A provider error (including a per-story token-ceiling abort) is not a gate failure
			// — treat as an unrecoverable discard.
			f.FailReasons[iter] = []string{fmt.Sprintf("provider error: %v", err)}
			f.TotalTokens = tokensOf(r.Provider) - startTokens
			return f
		}
		f.Candidates = append(f.Candidates, story)
		f.Iterations = iter
		prior = story

		res := checker.Check(story.Text)
		f.NewTokenTrace = append(f.NewTokenTrace, res.NewTokens)
		if res.Pass {
			f.Passed = true
			f.TotalTokens = tokensOf(r.Provider) - startTokens
			return f
		}
		f.FailReasons[iter] = res.Reasons
		f.FailCodes[iter] = res.Codes
		lastResult = res
	}
	f.TotalTokens = tokensOf(r.Provider) - startTokens
	return f // discarded after MaxIterations
}

// repairPrompt builds the rewrite prompt for one iteration: token-level name-and-replace when a
// lexicon is available, else the older count/type summary.
func (r *Reprocessor) repairPrompt(res gate.Result, b brief.Brief) string {
	if r.Lex != nil {
		return NameReplacePrompt(res, b, r.Lex, r.Band)
	}
	return RewritePrompt(res, b)
}

// tokensOf reports a provider's cumulative token usage, or 0 if it doesn't report any.
func tokensOf(p gen.Provider) int {
	if tr, ok := p.(interface{ TokensUsed() int }); ok {
		return tr.TokensUsed()
	}
	return 0
}

// gateChecker abstracts the segment→gate step so the repair loop is testable without a
// full pipeline. The real checker uses gate.Evaluate over a segmenter.
type gateChecker interface {
	Check(text string) gate.Result
}
