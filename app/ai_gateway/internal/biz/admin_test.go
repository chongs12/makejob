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

// failingAdminPromptRepoStub 模拟模板仓库不可用，验证自定义提示词可绕过模板依赖。
type failingAdminPromptRepoStub struct{}

// GetActiveTemplate 始终返回错误，模拟库里缺少对应场景模板。
func (r *failingAdminPromptRepoStub) GetActiveTemplate(context.Context, string) (*PromptTemplate, error) {
	return nil, errors.New("prompt not found")
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

// successLLMStub 返回固定 JSON，验证题卡候选生成主流程。
type successLLMStub struct{}

// Chat 返回最小合法题卡数组，便于断言自定义提示词路径可正常产出结果。
func (s *successLLMStub) Chat(context.Context, []Message, *AIConfig) (*LLMResponse, error) {
	return &LLMResponse{Content: `[{"title":"t","content":"c","type":"choice","difficulty":"easy","category":"Go","answer":"a","explanation":"e","tags":["go"]}]`}, nil
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

// TestGenerateQuestionCandidatesUsesCustomAgentPrompt 验证自定义 agent_prompt 存在时不会因模板缺失而失败。
func TestGenerateQuestionCandidatesUsesCustomAgentPrompt(t *testing.T) {
	uc := NewAdminUseCase(
		&adminConfigRepoStub{config: &AIConfig{Model: "test-model"}},
		&failingAdminPromptRepoStub{},
		&adminCallLogRepoStub{},
		&successLLMStub{},
		log.DefaultLogger,
	)

	result, err := uc.GenerateQuestionCandidates(
		context.Background(),
		"go_backend",
		"need go questions",
		"generate cards",
		3,
		"direct_single",
		true,
		true,
		[]string{"boss"},
		"Go后端",
		[]string{"Go基础", "并发编程"},
	)
	if err != nil {
		t.Fatalf("expected custom agent_prompt to bypass prompt repo error, got %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("expected one generated candidate, got %d", len(result.Candidates))
	}
}
