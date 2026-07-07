package gen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/lexicon"
)

// LLMConfig configures the OpenAI-compatible retelling client (handoff §4.2; DeepSeek per
// house pattern). The API key is read from the environment — never committed (I2) — and the
// network path is only reachable when a key is set and the caller explicitly opts in (the
// `zhuwenctl gen --live` flag). CI never sets a key, so it never takes the network path.
type LLMConfig struct {
	BaseURL     string  // e.g. https://api.deepseek.com/v1
	Model       string  // e.g. deepseek-chat
	APIKey      string  // from ZHUWEN_LLM_API_KEY
	Temperature float64 // low, for constrained retelling
	HTTPClient  *http.Client
}

// LLMConfigFromEnv reads the client config from the environment, falling back to the house
// key file ~/.deepseek-api-key when ZHUWEN_LLM_API_KEY is unset. Returns ok=false if no key is
// found anywhere, so callers can fall back to the fixture provider and CI stays hermetic.
func LLMConfigFromEnv() (LLMConfig, bool) {
	key := os.Getenv("ZHUWEN_LLM_API_KEY")
	if key == "" {
		if home, err := os.UserHomeDir(); err == nil {
			if b, err := os.ReadFile(filepath.Join(home, ".deepseek-api-key")); err == nil {
				key = strings.TrimSpace(string(b))
			}
		}
	}
	cfg := LLMConfig{
		BaseURL:     envOr("ZHUWEN_LLM_BASE_URL", "https://api.deepseek.com/v1"),
		Model:       envOr("ZHUWEN_LLM_MODEL", "deepseek-chat"),
		APIKey:      key,
		Temperature: 0.3,
	}
	return cfg, key != ""
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// LLMProvider retells briefs via a real LLM. It implements gen.Provider so the pipeline is
// unchanged; the app contains no generation code (I3) — this runs in the factory only.
type LLMProvider struct {
	cfg        LLMConfig
	lex        *lexicon.Lexicon
	tokens     int // cumulative total token usage across all calls (spike cost metric)
	promptTok  int // cumulative prompt (input) tokens — for accurate $-per-story
	completTok int // cumulative completion (output) tokens
}

// NewLLMProvider builds a provider. lex maps the brief's frontier/known word IDs to the actual
// characters the prompt lists.
func NewLLMProvider(cfg LLMConfig, lex *lexicon.Lexicon) *LLMProvider {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &LLMProvider{cfg: cfg, lex: lex}
}

// TokensUsed returns cumulative LLM token usage (for the spike's cost-per-story metric).
func (p *LLMProvider) TokensUsed() int { return p.tokens }

// PromptTokens and CompletionTokens return the cumulative input/output token split, so the
// spike report can price accepted stories against DeepSeek's differential input/output rates.
func (p *LLMProvider) PromptTokens() int     { return p.promptTok }
func (p *LLMProvider) CompletionTokens() int { return p.completTok }

// RepairProvider is a Provider that can regenerate with feedback from a failed gate result — the
// prior candidate plus a targeted rewrite prompt naming the exact violations (§4.4). The repair
// loop uses this when available so iterations actually converge instead of blindly retrying.
type RepairProvider interface {
	Provider
	RetellRepair(b brief.Brief, prior, repairPrompt string) (Story, error)
}

// RetellRepair regenerates the story given the previous (failed) candidate and a repair prompt.
// It sends the original brief contract, the prior attempt (as the assistant turn), then the
// repair instructions (as the next user turn) so the model edits rather than starts over.
func (p *LLMProvider) RetellRepair(b brief.Brief, prior, repairPrompt string) (Story, error) {
	if p.cfg.APIKey == "" {
		return Story{}, fmt.Errorf("gen: no API key; refusing network call")
	}
	msgs := p.BuildMessages(b)
	msgs = append(msgs,
		ChatMessage{Role: "assistant", Content: prior},
		ChatMessage{Role: "user", Content: repairPrompt})
	text, tokens, err := p.complete(msgs)
	if err != nil {
		return Story{}, err
	}
	p.tokens += tokens
	return Story{CanonID: b.CanonID, TitleZH: b.TitleZH, TitleEN: b.TitleEN,
		Band: b.Band, Register: b.Register, Text: text}, nil
}

// ChatMessage is one OpenAI-style chat message.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// BuildMessages compiles the §4.2 brief contract into a system+user prompt. Pure and
// deterministic (golden-tested) so prompt drift is caught without a network call. The prompt
// is constraint-first (CP-09a): it foregrounds the coverage budget, caps new words well below
// the gate's type budget to leave ratio headroom, forbids above-level vocabulary, and — for
// large frontier sets — describes the allowed level rather than dumping hundreds of words
// (which both bloats tokens and invites overuse).
func (p *LLMProvider) BuildMessages(b brief.Brief) []ChatMessage {
	system := "你是一位中文分级读物作者。你必须用严格受控的词汇改写故事梗概，" +
		"把词汇预算放在第一位：宁可句子简单、重复，也不要用超纲词。" +
		"只输出改写后的中文正文，不要输出任何解释、标题或正文以外的英文。"

	const promptNewTypeCap = 4 // guidance stricter than the gate's 8 → leaves coverage headroom
	const minRecurrence = 3

	var sb strings.Builder
	fmt.Fprintf(&sb, "故事：%s（%s）\n", b.TitleZH, b.TitleEN)
	fmt.Fprintf(&sb, "语级：%s（HSK %d，语域：%s）\n", b.Band, b.HSK3Level, b.Register)
	fmt.Fprintf(&sb, "长度：请写 %d–%d 字，尽量接近上限。把每个情节展开叙述，多用重复的简单句——文章越长，生词比例越容易达标。\n\n", b.LengthMin, b.LengthMax)

	sb.WriteString("情节梗概（必须逐条覆盖）：\n")
	for i, beat := range b.Beats {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, beat)
	}
	if len(b.Characters) > 0 {
		sb.WriteString("\n人物（只能用这些名字；名字首次出现时用一句话解释）：")
		names := make([]string, 0, len(b.Characters))
		for _, c := range b.Characters {
			if c.Gloss != "" {
				names = append(names, fmt.Sprintf("%s（%s）", c.NameZH, c.Gloss))
			} else {
				names = append(names, c.NameZH)
			}
		}
		sb.WriteString(strings.Join(names, "、"))
		sb.WriteString("\n")
	}

	sb.WriteString("\n词汇规则（最重要，必须严格遵守）：\n")
	fmt.Fprintf(&sb, "· 全文至少 98%% 的词必须是 HSK %d 及以下的常用词。\n", b.HSK3Level)
	fmt.Fprintf(&sb, "· 绝对不能使用 HSK %d 以上的词。宁可换一个简单的说法。\n", b.HSK3Level+1)

	// CP-09b coverage-contract: when PlanNewTypes is set, list ONLY that small chosen
	// set of ≤promptNewTypeCap frontier words — the deliberate per-story contract that
	// improves the accept rate by focusing the LLM on achievable targets (09a weakness).
	// Otherwise fall back to the old frontier-dump heuristic (≤30 words) or the
	// level-description fallback (>30 words).
	plan := b.PlanNewTypes
	if len(plan) > 0 {
		chosen := p.simpsForIDs(plan)
		fmt.Fprintf(&sb, "· 全篇最多引入 %d 个新词，每个新词必须至少出现 %d 次。\n",
			len(plan), b.MinRecurrence)
		if len(chosen) > 0 {
			fmt.Fprintf(&sb, "· 新词只能精确地使用以下 %d 个词（每个至少出现 %d 次）：%s\n",
				len(chosen), b.MinRecurrence, strings.Join(chosen, "、"))
		}
	} else {
		fmt.Fprintf(&sb, "· 全篇最多引入 %d 个新词（HSK %d 级），每个新词必须至少出现 %d 次。\n",
			promptNewTypeCap, b.HSK3Level+1, minRecurrence)
		frontier := p.simpsFor(b.Frontier)
		switch {
		case len(frontier) == 0:
			sb.WriteString("· 本篇不得引入任何新词，只能用已知词。\n")
		case len(frontier) <= 30:
			fmt.Fprintf(&sb, "· 新词只能从这些里选：%s\n", strings.Join(frontier, "、"))
		default:
			fmt.Fprintf(&sb, "· 新词必须是 HSK %d 级的常用词（不要用生僻词）。\n", b.HSK3Level+1)
		}
	}
	sb.WriteString("· 不得使用生僻字或未解释的专有名词。\n")
	sb.WriteString("· 多用重复、简单短句；同一个意思尽量用同一个已知词表达。\n")
	sb.WriteString("\n请直接输出改写后的中文正文：")

	return []ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: sb.String()},
	}
}

// simpsFor maps a set of word IDs to their simplified forms, sorted by ID for determinism.
func (p *LLMProvider) simpsFor(ids map[int]bool) []string {
	list := make([]int, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	sort.Ints(list)
	var out []string
	for _, id := range list {
		if w, ok := p.lex.LookupID(id); ok {
			out = append(out, w.Simp)
		}
	}
	return out
}

// simpsForIDs maps a sorted list of word IDs to their simplified forms.
func (p *LLMProvider) simpsForIDs(ids []int) []string {
	var out []string
	for _, id := range ids {
		if w, ok := p.lex.LookupID(id); ok {
			out = append(out, w.Simp)
		}
	}
	return out
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// parseCompletion extracts the retold text (and token usage) from an OpenAI-style response
// body. Pure and hermetically tested against canned bodies.
func parseCompletion(body []byte) (text string, totalTokens int, err error) {
	t, total, _, _, err := parseCompletionSplit(body)
	return t, total, err
}

// parseCompletionSplit is parseCompletion plus the prompt/completion token split.
func parseCompletionSplit(body []byte) (text string, total, prompt, completion int, err error) {
	var resp chatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", 0, 0, 0, fmt.Errorf("gen: bad response body: %w", err)
	}
	if resp.Error != nil {
		return "", 0, 0, 0, fmt.Errorf("gen: api error: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", 0, 0, 0, fmt.Errorf("gen: no choices in response")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content),
		resp.Usage.TotalTokens, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, nil
}

// Retell calls the LLM to produce one candidate. NETWORK PATH — requires an API key; returns
// an error (never a silent no-op) if unconfigured, so CI can never accidentally reach the wire.
func (p *LLMProvider) Retell(b brief.Brief) (Story, error) {
	story, _, err := p.RetellWithUsage(b)
	return story, err
}

// RetellWithUsage is Retell plus the token count (for the spike's cost metric).
func (p *LLMProvider) RetellWithUsage(b brief.Brief) (Story, int, error) {
	if p.cfg.APIKey == "" {
		return Story{}, 0, fmt.Errorf("gen: no API key (set ZHUWEN_LLM_API_KEY); refusing network call")
	}
	text, tokens, err := p.complete(p.BuildMessages(b))
	if err != nil {
		return Story{}, 0, err
	}
	p.tokens += tokens
	return Story{
		CanonID:  b.CanonID,
		TitleZH:  b.TitleZH,
		TitleEN:  b.TitleEN,
		Band:     b.Band,
		Register: b.Register,
		Text:     text,
		Fixture:  false,
	}, tokens, nil
}

// complete posts a chat-completion request and returns the trimmed content + token usage.
func (p *LLMProvider) complete(msgs []ChatMessage) (string, int, error) {
	reqBody, err := json.Marshal(chatRequest{
		Model:       p.cfg.Model,
		Messages:    msgs,
		Temperature: p.cfg.Temperature,
	})
	if err != nil {
		return "", 0, err
	}
	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

	resp, err := p.cfg.HTTPClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return "", 0, err
	}
	text, total, prompt, completion, err := parseCompletionSplit(buf.Bytes())
	if err != nil {
		return "", 0, err
	}
	p.promptTok += prompt
	p.completTok += completion
	return text, total, nil
}
