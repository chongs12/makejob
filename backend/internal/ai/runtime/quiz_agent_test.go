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

// stubQuizAnalyzer 为测试提供可预测的回退结果。
type stubQuizAnalyzer struct {
	analysis    ai.CodeAnalysis
	explanation string
	hint        string
}

// AnalyzeCode 返回预设分析结果。
func (a *stubQuizAnalyzer) AnalyzeCode(context.Context, string, string, string) (ai.CodeAnalysis, error) {
	return a.analysis, nil
}

// ExplainAnswer 返回预设题解结果。
func (a *stubQuizAnalyzer) ExplainAnswer(context.Context, string, string, string) (string, error) {
	return a.explanation, nil
}

// GenerateHint 返回预设提示结果。
func (a *stubQuizAnalyzer) GenerateHint(context.Context, string, string) (string, error) {
	return a.hint, nil
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

// TestProviderQuizAnalyzerAnalyzeCodeFallsBackOnInvalidJSON 验证坏格式输出会回退到兜底实现。
func TestProviderQuizAnalyzerAnalyzeCodeFallsBackOnInvalidJSON(t *testing.T) {
	analyzer := &providerQuizAnalyzer{
		provider: &staticProvider{response: "not-json"},
		fallback: &stubQuizAnalyzer{
			analysis: ai.CodeAnalysis{
				IsCorrect: true,
				Score:     88,
				Feedback:  "fallback-analysis",
			},
		},
	}

	result, err := analyzer.AnalyzeCode(context.Background(), "func main() {}", "go", "实现一个示例函数")
	if err != nil {
		t.Fatalf("AnalyzeCode returned error: %v", err)
	}

	if result.Feedback != "fallback-analysis" {
		t.Fatalf("expected fallback analysis, got %q", result.Feedback)
	}
	if !result.IsCorrect {
		t.Fatalf("expected fallback correctness to be kept")
	}
}

// TestProviderQuizAnalyzerExplainAnswerFallsBackOnEmptyResponse 验证空文本输出会回退到兜底题解。
func TestProviderQuizAnalyzerExplainAnswerFallsBackOnEmptyResponse(t *testing.T) {
	analyzer := &providerQuizAnalyzer{
		provider: &staticProvider{response: "   "},
		fallback: &stubQuizAnalyzer{
			explanation: "fallback-explanation",
		},
	}

	result, err := analyzer.ExplainAnswer(context.Background(), "两数之和", "给定数组，返回下标。", "哈希表")
	if err != nil {
		t.Fatalf("ExplainAnswer returned error: %v", err)
	}

	if result != "fallback-explanation" {
		t.Fatalf("expected fallback explanation, got %q", result)
	}
}
