package runtime

import (
	"context"
	"strings"
	"testing"

	"makejob-backend/internal/ai"
)

// structuredTestPayload 用于验证结构化输出修复流程。
type structuredTestPayload struct {
	Value string `json:"value"`
}

// sequenceProvider 按调用顺序返回预设响应，便于测试修复流程。
type sequenceProvider struct {
	responses []string
	errs      []error
	callCount int
}

// Chat 按顺序返回测试响应。
func (p *sequenceProvider) Chat(context.Context, []ai.Message) (*ai.ChatResponse, error) {
	index := p.callCount
	p.callCount++

	if index < len(p.errs) && p.errs[index] != nil {
		return nil, p.errs[index]
	}
	if len(p.responses) == 0 {
		return &ai.ChatResponse{}, nil
	}
	if index >= len(p.responses) {
		return &ai.ChatResponse{Content: p.responses[len(p.responses)-1]}, nil
	}
	return &ai.ChatResponse{Content: p.responses[index]}, nil
}

// StreamChat 返回空流实现接口。
func (p *sequenceProvider) StreamChat(context.Context, []ai.Message) (<-chan string, error) {
	return nil, nil
}

// GetModelName 返回测试模型名。
func (p *sequenceProvider) GetModelName() string {
	return "sequence-provider"
}

// TestCallStructuredJSONRepairsInvalidResponse 验证首次非 JSON 时会尝试二次修复。
func TestCallStructuredJSONRepairsInvalidResponse(t *testing.T) {
	provider := &sequenceProvider{
		responses: []string{
			"这是一段解释，不是 JSON",
			`{"value":"repaired"}`,
		},
	}

	payload, trace, _, err := callStructuredJSON[structuredTestPayload](context.Background(), provider, []ai.Message{
		{Role: "user", Content: "test"},
	}, `{"value":"字符串"}`)
	if err != nil {
		t.Fatalf("callStructuredJSON returned error: %v", err)
	}
	if payload.Value != "repaired" {
		t.Fatalf("expected repaired payload, got %q", payload.Value)
	}
	if provider.callCount != 2 {
		t.Fatalf("expected provider to be called twice, got %d", provider.callCount)
	}
	if !strings.Contains(trace, "[initial_response]") || !strings.Contains(trace, "[repair_response]") {
		t.Fatalf("expected trace to contain both responses, got %q", trace)
	}
}

// TestCallStructuredJSONReturnsErrorWhenRepairFails 验证二次修复仍失败时返回完整错误。
func TestCallStructuredJSONReturnsErrorWhenRepairFails(t *testing.T) {
	provider := &sequenceProvider{
		responses: []string{
			"not-json",
			"still-not-json",
		},
	}

	_, trace, _, err := callStructuredJSON[structuredTestPayload](context.Background(), provider, []ai.Message{
		{Role: "user", Content: "test"},
	}, `{"value":"字符串"}`)
	if err == nil {
		t.Fatalf("expected error when repair still fails")
	}
	if provider.callCount != 2 {
		t.Fatalf("expected provider to be called twice, got %d", provider.callCount)
	}
	if !strings.Contains(err.Error(), "repair decode failed") {
		t.Fatalf("expected repair decode error, got %v", err)
	}
	if !strings.Contains(trace, "[initial_response]") || !strings.Contains(trace, "[repair_response]") {
		t.Fatalf("expected trace to contain both responses, got %q", trace)
	}
}

// TestProviderQuizAnalyzerAnalyzeCodeRepairsInvalidJSON 验证判题链路在修复成功后不会回退到 mock。
func TestProviderQuizAnalyzerAnalyzeCodeRepairsInvalidJSON(t *testing.T) {
	provider := &sequenceProvider{
		responses: []string{
			"先给你一段说明文字",
			`{"is_correct":true,"score":86,"feedback":"repaired-analysis","issues":["边界条件说明不足"],"improvements":["补充复杂度分析"],"time_complexity":"O(n)","space_complexity":"O(1)"}`,
		},
	}
	analyzer := &providerQuizAnalyzer{
		provider: provider,
	}

	result, err := analyzer.AnalyzeCode(context.Background(), "func main() {}", "go", "分析这段代码")
	if err != nil {
		t.Fatalf("AnalyzeCode returned error: %v", err)
	}
	if provider.callCount != 2 {
		t.Fatalf("expected provider to be called twice, got %d", provider.callCount)
	}
	if result.Feedback != "repaired-analysis" {
		t.Fatalf("expected repaired analysis, got %q", result.Feedback)
	}
}
