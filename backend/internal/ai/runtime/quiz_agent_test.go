package runtime

import (
	"context"
	"testing"

	"makejob-backend/internal/ai"
)

// staticProvider 为测试返回固定模型响应。
type staticProvider struct {
	response string
	err      error
}

// Chat 返回预设响应。
func (p *staticProvider) Chat(context.Context, []ai.Message) (string, error) {
	return p.response, p.err
}

// StreamChat 为测试桩返回空实现。
func (p *staticProvider) StreamChat(context.Context, []ai.Message) (<-chan string, error) {
	return nil, nil
}

// GetModelName 返回测试模型名。
func (p *staticProvider) GetModelName() string {
	return "test-provider"
}

// TestNormalizeQuizAnalysisFillsDefaults 验证模型缺字段时会补齐默认值。
func TestNormalizeQuizAnalysisFillsDefaults(t *testing.T) {
	isCorrect := true
	result := normalizeQuizAnalysis(quizAnalysisPayload{
		IsCorrect: &isCorrect,
		Feedback:  "回答方向正确。",
	}, "这是一个较完整的主观题答案，用来验证缺省字段补齐逻辑。", "text", "解释 CAP 定理。")

	if !result.IsCorrect {
		t.Fatalf("expected normalized result to be correct")
	}
	if result.Score < 60 {
		t.Fatalf("expected normalized score to be filled, got %v", result.Score)
	}
	if len(result.Issues) == 0 {
		t.Fatalf("expected default issues to be filled")
	}
	if len(result.Improvements) == 0 {
		t.Fatalf("expected default improvements to be filled")
	}
	if result.TimeComplexity == "" || result.SpaceComplexity == "" {
		t.Fatalf("expected complexity fields to be filled")
	}
}

// TestProviderQuizAnalyzerAnalyzeCodeReturnsErrorOnInvalidJSON 验证坏格式输出会直接返回错误。
func TestProviderQuizAnalyzerAnalyzeCodeReturnsErrorOnInvalidJSON(t *testing.T) {
	analyzer := &providerQuizAnalyzer{
		provider: &staticProvider{response: "not-json"},
	}

	result, err := analyzer.AnalyzeCode(context.Background(), "func main() {}", "go", "实现一个示例函数")
	if err == nil {
		t.Fatalf("expected AnalyzeCode to return error")
	}
	if result.Feedback != "" || result.Score != 0 || result.IsCorrect || len(result.Issues) != 0 || len(result.Improvements) != 0 {
		t.Fatalf("expected zero-value analysis on error, got %#v", result)
	}
}

// TestProviderQuizAnalyzerExplainAnswerReturnsErrorOnEmptyResponse 验证空文本输出会直接返回错误。
func TestProviderQuizAnalyzerExplainAnswerReturnsErrorOnEmptyResponse(t *testing.T) {
	analyzer := &providerQuizAnalyzer{
		provider: &staticProvider{response: "   "},
	}

	result, err := analyzer.ExplainAnswer(context.Background(), "两数之和", "给定数组，返回下标。", "哈希表")
	if err == nil {
		t.Fatalf("expected ExplainAnswer to return error")
	}
	if result != "" {
		t.Fatalf("expected empty explanation on error, got %q", result)
	}
}
