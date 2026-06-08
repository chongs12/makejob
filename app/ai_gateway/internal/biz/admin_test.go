package biz

import (
	"context"
	"errors"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
)

// adminConfigRepoStub 为 AI Gateway 管理调试测试提供最小配置仓库桩。
type adminConfigRepoStub struct {
	config *AIConfig
}

// GetActiveConfig 返回预置配置。
func (r *adminConfigRepoStub) GetActiveConfig(context.Context, string) (*AIConfig, error) {
	return r.config, nil
}

// adminPromptRepoStub 为 Prompt 渲染测试提供模板仓库桩。
type adminPromptRepoStub struct{}

// GetActiveTemplate 返回固定模板。
func (r *adminPromptRepoStub) GetActiveTemplate(context.Context, string) (*PromptTemplate, error) {
	return &PromptTemplate{TemplateText: "hello {{name}}"}, nil
}

// adminCallLogRepoStub 吞掉日志写入，避免干扰测试断言。
type adminCallLogRepoStub struct{}

// Create 返回空结果，满足接口要求。
func (r *adminCallLogRepoStub) Create(context.Context, *AICallLog) error {
	return nil
}

// failingLLMStub 模拟下游 LLM 调用失败。
type failingLLMStub struct{}

// Chat 始终返回错误，验证 RenderPrompt 不会伪成功。
func (f *failingLLMStub) Chat(context.Context, []Message, *AIConfig) (*LLMResponse, error) {
	return nil, errors.New("llm unavailable")
}

// TestRenderPromptReturnsErrorWhenLLMFails 验证 Prompt 试跑失败时会返回明确错误码，而不是把错误文本塞进成功响应。
func TestRenderPromptReturnsErrorWhenLLMFails(t *testing.T) {
	uc := NewAdminUseCase(
		&adminConfigRepoStub{config: &AIConfig{Model: "test-model"}},
		&adminPromptRepoStub{},
		&adminCallLogRepoStub{},
		&failingLLMStub{},
		log.DefaultLogger,
	)

	result, err := uc.RenderPrompt(context.Background(), "scene", "hello {{name}}", map[string]string{"name": "world"}, true)
	if err == nil {
		t.Fatalf("expected RenderPrompt to return error when LLM fails")
	}
	if result != nil {
		t.Fatalf("expected no success result on LLM failure, got %+v", result)
	}
}
