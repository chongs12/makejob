package data

import (
	"context"
	"testing"
	"time"

	"makejob/app/ai_gateway/internal/biz"
)

type testContextKey string

// newTimeoutConfig 构造仅包含超时参数的测试 AI 配置。
func newTimeoutConfig(timeoutSeconds string) *biz.AIConfig {
	return &biz.AIConfig{
		ExtraParamsJSON: `{"ai_timeout_seconds":"` + timeoutSeconds + `"}`,
	}
}

// TestEffectiveLLMRequestTimeoutPrefersAdminConfig 验证模型调用优先使用后台 AI 配置中的超时值。
func TestEffectiveLLMRequestTimeoutPrefersAdminConfig(t *testing.T) {
	timeout := effectiveLLMRequestTimeout(120*time.Second, newTimeoutConfig("45"))
	if timeout != 45*time.Second {
		t.Fatalf("expected 45s timeout, got %s", timeout)
	}
}

// TestBuildLLMRequestContextIgnoresParentDeadline 验证模型调用上下文不会继承上游过短 deadline，但会保留上下文值。
func TestBuildLLMRequestContextIgnoresParentDeadline(t *testing.T) {
	parentKey := testContextKey("trace")
	parentBase := context.WithValue(context.Background(), parentKey, "trace-123")
	parent, parentCancel := context.WithTimeout(parentBase, time.Second)
	defer parentCancel()

	requestCtx, cancel := buildLLMRequestContext(parent, 45*time.Second)
	defer cancel()

	deadline, ok := requestCtx.Deadline()
	if !ok {
		t.Fatalf("expected request context to have deadline")
	}
	remaining := time.Until(deadline)
	if remaining < 40*time.Second {
		t.Fatalf("expected request deadline to be detached from 1s parent deadline, got %s", remaining)
	}
	if value := requestCtx.Value(parentKey); value != "trace-123" {
		t.Fatalf("expected context value to be preserved, got %#v", value)
	}

	parentCancel()
	if err := requestCtx.Err(); err != nil {
		t.Fatalf("expected detached request context to ignore parent cancel, got %v", err)
	}
}
