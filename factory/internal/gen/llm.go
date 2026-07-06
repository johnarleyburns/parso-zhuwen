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
	cfg    LLMConfig
	lex    *lexicon.Lexicon
	tokens int // cumulative token usage across all Retell calls (spike cost metric)
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
// deterministic (golden-tested) so prompt drift is caught without a network call.
func (p *LLMProvider) BuildMessages(b brief.Brief) []ChatMessage {
	system := "你是一位中文分级读物作者。你必须用受控词汇改写给定的故事梗概，" +
		"严格遵守词汇预算。只输出改写后的中文正文，不要输出任何解释、标题或标点以外的英文。"

	var sb strings.Builder
	fmt.Fprintf(&sb, "故事：%s（%s）\n", b.TitleZH, b.TitleEN)
	fmt.Fprintf(&sb, "语级：%s（HSK %d，语域：%s）\n", b.Band, b.HSK3Level, b.Register)
	fmt.Fprintf(&sb, "长度：%d–%d 字。\n\n", b.LengthMin, b.LengthMax)

	sb.WriteString("情节梗概（必须逐条覆盖）：\n")
	for i, beat := range b.Beats {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, beat)
	}
	if len(b.Characters) > 0 {
		sb.WriteString("\n人物：")
		names := make([]string, 0, len(b.Characters))
		for _, c := range b.Characters {
			names = append(names, c.NameZH)
		}
		sb.WriteString(strings.Join(names, "、"))
		sb.WriteString("\n")
	}

	sb.WriteString("\n词汇规则：\n")
	fmt.Fprintf(&sb, "· 只能使用 HSK %d 及以下的已知词。\n", b.HSK3Level)
	fmt.Fprintf(&sb, "· 新词（超出已知范围）最多 %d 个词型，且每个新词至少出现 3 次。\n", 8)
	sb.WriteString("· 新词只能从下面的“允许新词”列表中选取：\n")
	frontier := p.simpsFor(b.Frontier)
	if len(frontier) == 0 {
		sb.WriteString("  （无——不得引入任何新词）\n")
	} else {
		fmt.Fprintf(&sb, "  %s\n", strings.Join(frontier, "、"))
	}
	sb.WriteString("· 不得使用生僻字或专有名词（除非在梗概中已给出并在首次出现时解释）。\n")
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
	var resp chatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", 0, fmt.Errorf("gen: bad response body: %w", err)
	}
	if resp.Error != nil {
		return "", 0, fmt.Errorf("gen: api error: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", 0, fmt.Errorf("gen: no choices in response")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), resp.Usage.TotalTokens, nil
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
	return parseCompletion(buf.Bytes())
}
