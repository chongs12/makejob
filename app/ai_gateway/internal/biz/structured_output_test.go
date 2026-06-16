package biz

import (
	"context"
	"testing"
)

// seqLLMClient 按调用顺序返回预设响应，用于验证 JSON 修复重试。
type seqLLMClient struct {
	responses []*LLMResponse
	errs      []error
	calls     int
}

func (m *seqLLMClient) Chat(ctx context.Context, messages []Message, config *AIConfig) (*LLMResponse, error) {
	i := m.calls
	m.calls++
	var resp *LLMResponse
	var err error
	if i < len(m.responses) {
		resp = m.responses[i]
	}
	if i < len(m.errs) {
		err = m.errs[i]
	}
	return resp, err
}

func TestExtractJSONObject(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"raw", `{"a":1}`, `{"a":1}`},
		{"fenced", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"prose_wrapped", "好的，结果如下：\n{\"a\":1}\n以上。", `{"a":1}`},
		{"no_json", "纯文本没有 JSON", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extractJSONObject(c.in); got != c.want {
				t.Errorf("extractJSONObject(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestParseStructuredJSON_FirstTrySucceedsWithFence(t *testing.T) {
	cfg := &AIConfig{Model: "test"}
	llm := &seqLLMClient{}
	schema := quizResultSchema()
	raw := "```json\n{\"score\":90,\"is_correct\":true,\"feedback\":\"好\"}\n```"

	result, err := parseStructuredJSON[QuizResult](context.Background(), llm, cfg, raw, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Score != 90 || !result.IsCorrect {
		t.Errorf("unexpected result: %+v", result)
	}
	if llm.calls != 0 {
		t.Errorf("expected no repair call, got %d", llm.calls)
	}
}

func TestParseStructuredJSON_RepairRetry(t *testing.T) {
	cfg := &AIConfig{Model: "test"}
	// 首次输出是无法解析的散文，修复请求返回合法 JSON。
	llm := &seqLLMClient{
		responses: []*LLMResponse{
			{Content: `{"plan_title":"Go 进阶","tasks":[],"summary":"总结"}`},
		},
	}
	schema := planResultSchema()
	raw := "这是一段没有 JSON 的散文回复。"

	result, err := parseStructuredJSON[PlanResult](context.Background(), llm, cfg, raw, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PlanTitle != "Go 进阶" {
		t.Errorf("expected repaired plan_title, got %q", result.PlanTitle)
	}
	if llm.calls != 1 {
		t.Errorf("expected exactly 1 repair call, got %d", llm.calls)
	}
}

func TestParseStructuredJSON_RepairAlsoFails(t *testing.T) {
	cfg := &AIConfig{Model: "test"}
	llm := &seqLLMClient{
		responses: []*LLMResponse{
			{Content: "修复也没给出 JSON"},
		},
	}
	schema := planResultSchema()

	_, err := parseStructuredJSON[PlanResult](context.Background(), llm, cfg, "散文", schema)
	if err != ErrParseFailed {
		t.Fatalf("expected ErrParseFailed, got %v", err)
	}
}
