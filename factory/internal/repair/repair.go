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
	"strings"

	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/gen"
)

// MaxIterations is the cap on repair attempts before discarding (§4.4).
const MaxIterations = 4

// Fate records the outcome of a repair cycle (used in the spike report).
type Fate struct {
	Brief       brief.Brief
	Iterations  int
	Passed      bool
	Candidates  []gen.Story
	FailReasons [][]string // per-iteration failure reasons
	FailCodes   [][]string // per-iteration stable failure codes (for the histogram)
	TotalTokens int        // cumulative LLM token usage across all attempts
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

// Reprocessor orchestrates the retell → gate → repair loop for one brief. It is bound to a
// provider (real LLM or fixture) and a segmenter + band so the loop can run isolated.
type Reprocessor struct {
	Provider gen.Provider
}

// NewReprocessor builds a repair orchestrator over the given provider.
func NewReprocessor(p gen.Provider) *Reprocessor {
	return &Reprocessor{Provider: p}
}

// Run iterates: retell → produce failure hints → retell with hint — up to MaxIterations.
// If the LLM provider is used it accumulates token usage for the spike's cost-per-story
// metric; for a fixture provider this is zero.
func (r *Reprocessor) Run(b brief.Brief, checker gateChecker) Fate {
	var f Fate
	f.Brief = b
	f.Candidates = make([]gen.Story, 0, MaxIterations+1)
	f.FailReasons = make([][]string, MaxIterations+1)
	f.FailCodes = make([][]string, MaxIterations+1)
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
			story, err = rp.RetellRepair(b, prior.Text, RewritePrompt(lastResult, b))
		} else {
			story, err = r.Provider.Retell(b)
		}
		if err != nil {
			// A provider error is not a gate failure — treat as unrecoverable discard.
			f.FailReasons[iter] = []string{fmt.Sprintf("provider error: %v", err)}
			return f
		}
		f.Candidates = append(f.Candidates, story)
		f.Iterations = iter
		prior = story

		res := checker.Check(story.Text)
		if res.Pass {
			f.Passed = true
			return f
		}
		f.FailReasons[iter] = res.Reasons
		f.FailCodes[iter] = res.Codes
		lastResult = res
	}
	return f // discarded after MaxIterations
}

// gateChecker abstracts the segment→gate step so the repair loop is testable without a
// full pipeline. The real checker uses gate.Evaluate over a segmenter.
type gateChecker interface {
	Check(text string) gate.Result
}
