package gen

import (
	"strings"
	"testing"

	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/canon"
	"github.com/parso/zhuwen-factory/internal/lexicon"
)

func testLexicon(t *testing.T) *lexicon.Lexicon {
	t.Helper()
	src := "1\t山\tshān\t1\t10\t\n2\t水\tshuǐ\t1\t11\t\n3\t坚持\tjiānchí\t3\t900\t\n"
	lex, err := lexicon.Ingest(strings.NewReader(src), "gen-test-v1")
	if err != nil {
		t.Fatalf("lexicon: %v", err)
	}
	return lex
}

func TestBuildMessagesEncodesBriefContract(t *testing.T) {
	lex := testLexicon(t)
	p := NewLLMProvider(LLMConfig{Model: "deepseek-chat"}, lex)
	b := brief.Brief{
		CanonID: "c1", TitleZH: "拔苗助长", TitleEN: "Pulling Up Seedlings",
		Band: "A2", HSK3Level: 2, Register: "narrative",
		Beats:      []string{"农夫种田", "农夫拔苗", "苗都死了"},
		Characters: []canon.Character{{NameZH: "农夫"}},
		Frontier:   map[int]bool{3: true},
		LengthMin:  120, LengthMax: 400,
	}

	msgs := p.BuildMessages(b)
	if len(msgs) != 2 || msgs[0].Role != "system" || msgs[1].Role != "user" {
		t.Fatalf("messages shape wrong: %+v", msgs)
	}
	user := msgs[1].Content
	for _, want := range []string{"拔苗助长", "HSK 2", "农夫种田", "坚持", "至少出现 3 次", "120–400"} {
		if !strings.Contains(user, want) {
			t.Errorf("user prompt missing %q\n---\n%s", want, user)
		}
	}
	// Determinism: same brief → identical prompt (no map-iteration drift).
	msgs2 := p.BuildMessages(b)
	if msgs2[1].Content != user {
		t.Error("BuildMessages is not deterministic")
	}
}

func TestBuildMessagesNoFrontierForbidsNewWords(t *testing.T) {
	lex := testLexicon(t)
	p := NewLLMProvider(LLMConfig{}, lex)
	b := brief.Brief{TitleZH: "t", Beats: []string{"x"}, Frontier: map[int]bool{}}
	user := p.BuildMessages(b)[1].Content
	if !strings.Contains(user, "不得引入任何新词") {
		t.Errorf("empty frontier should forbid new words:\n%s", user)
	}
}

func TestParseCompletion(t *testing.T) {
	body := `{"choices":[{"message":{"role":"assistant","content":"  山水山水。  "}}],"usage":{"total_tokens":42}}`
	text, tokens, err := parseCompletion([]byte(body))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if text != "山水山水。" {
		t.Errorf("text = %q, want trimmed 山水山水。", text)
	}
	if tokens != 42 {
		t.Errorf("tokens = %d, want 42", tokens)
	}
}

func TestParseCompletionErrors(t *testing.T) {
	if _, _, err := parseCompletion([]byte(`{"error":{"message":"bad key"}}`)); err == nil {
		t.Error("expected api error")
	}
	if _, _, err := parseCompletion([]byte(`{"choices":[]}`)); err == nil {
		t.Error("expected no-choices error")
	}
	if _, _, err := parseCompletion([]byte(`not json`)); err == nil {
		t.Error("expected malformed-body error")
	}
}

func TestRetellRefusesWithoutKey(t *testing.T) {
	// CI safety: no key → hard error, never a network call.
	p := NewLLMProvider(LLMConfig{}, testLexicon(t))
	if _, err := p.Retell(brief.Brief{}); err == nil {
		t.Fatal("expected refusal without API key")
	}
}
