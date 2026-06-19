package biz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
)

// mockAIConfigRepo 模拟 AI 配置仓库
type mockAIConfigRepo struct {
	config *AIConfig
	err    error
}

func (m *mockAIConfigRepo) GetActiveConfig(ctx context.Context, scene string) (*AIConfig, error) {
	return m.config, m.err
}

// mockPromptRepo 模拟 Prompt 模板仓库
type mockPromptRepo struct {
	template *PromptTemplate
	err      error
}

func (m *mockPromptRepo) GetActiveTemplate(ctx context.Context, scene string) (*PromptTemplate, error) {
	return m.template, m.err
}

// mockCallLogRepo 模拟调用日志仓库
type mockCallLogRepo struct {
	createErr error
}

func (m *mockCallLogRepo) Create(ctx context.Context, log *AICallLog) error {
	return m.createErr
}

// mockLLMClient 模拟 LLM 客户端
type mockLLMClient struct {
	response *LLMResponse
	err      error
}

func (m *mockLLMClient) Chat(ctx context.Context, messages []Message, config *AIConfig) (*LLMResponse, error) {
	return m.response, m.err
}

func TestInterviewAgentUseCase_GenerateQuestion_Success(t *testing.T) {
	configRepo := &mockAIConfigRepo{
		config: &AIConfig{Scene: "interview_agent", Model: "test-model"},
	}
	promptRepo := &mockPromptRepo{
		template: &PromptTemplate{TemplateContent: "面试题目：{{industry_code}}"},
	}
	callLogRepo := &mockCallLogRepo{}
	llmClient := &mockLLMClient{
		response: &LLMResponse{
			Content:      `{"question":"什么是Goroutine?","topic":"并发","difficulty":"medium","type":"technical"}`,
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	uc := NewInterviewAgentUseCase(configRepo, promptRepo, callLogRepo, llmClient, log.DefaultLogger)

	result, err := uc.GenerateQuestion(context.Background(), "golang", "medium", "", "", "", nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Question != "什么是Goroutine?" {
		t.Errorf("expected question='什么是Goroutine?', got '%s'", result.Question)
	}
	if result.Topic != "并发" {
		t.Errorf("expected topic='并发', got '%s'", result.Topic)
	}
}

func TestInterviewAgentUseCase_GenerateQuestion_ConfigNotFound(t *testing.T) {
	configRepo := &mockAIConfigRepo{
		err: ErrAIConfigNotFound,
	}
	promptRepo := &mockPromptRepo{}
	callLogRepo := &mockCallLogRepo{}
	llmClient := &mockLLMClient{}

	uc := NewInterviewAgentUseCase(configRepo, promptRepo, callLogRepo, llmClient, log.DefaultLogger)

	_, err := uc.GenerateQuestion(context.Background(), "golang", "medium", "", "", "", nil, 1)
	if err != ErrAIConfigNotFound {
		t.Errorf("expected ErrAIConfigNotFound, got %v", err)
	}
}

func TestPlanAgentUseCase_GeneratePlan_Success(t *testing.T) {
	configRepo := &mockAIConfigRepo{
		config: &AIConfig{Scene: "plan_agent", Model: "test-model"},
	}
	promptRepo := &mockPromptRepo{
		template: &PromptTemplate{TemplateContent: "学习计划：{{industry_code}}"},
	}
	callLogRepo := &mockCallLogRepo{}
	llmClient := &mockLLMClient{
		response: &LLMResponse{
			Content:      `{"plan_title":"Go学习计划","tasks":[{"title":"基础语法","description":"学习Go基础","phase":"phase1","order_index":1,"estimated_hours":10}],"summary":"30天掌握Go"}`,
			InputTokens:  200,
			OutputTokens: 100,
		},
	}

	uc := NewPlanAgentUseCase(configRepo, promptRepo, callLogRepo, llmClient, log.DefaultLogger)

	result, err := uc.GeneratePlan(context.Background(), "golang", "掌握Go", 2, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PlanTitle != "Go学习计划" {
		t.Errorf("expected plan_title='Go学习计划', got '%s'", result.PlanTitle)
	}
	if len(result.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(result.Tasks))
	}
}

func TestCompanionAgentUseCase_Chat_Success(t *testing.T) {
	configRepo := &mockAIConfigRepo{
		config: &AIConfig{Scene: "companion_agent", Model: "test-model"},
	}
	promptRepo := &mockPromptRepo{
		template: &PromptTemplate{TemplateContent: "陪伴聊天：{{user_message}}"},
	}
	callLogRepo := &mockCallLogRepo{}
	llmClient := &mockLLMClient{
		response: &LLMResponse{
			Content:      "你好呀，今天想学点什么？",
			InputTokens:  50,
			OutputTokens: 30,
		},
	}

	uc := NewCompanionAgentUseCase(configRepo, promptRepo, callLogRepo, llmClient, log.DefaultLogger)

	// 陪伴回复为纯文本，直接作为 reply；emotion 由 contextType 本地推导。
	result, err := uc.Chat(context.Background(), "你好", "happy", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Reply != "你好呀，今天想学点什么？" {
		t.Errorf("expected reply='你好呀，今天想学点什么？', got '%s'", result.Reply)
	}
	if result.Emotion != "happy" {
		t.Errorf("expected emotion='happy', got '%s'", result.Emotion)
	}
	if result.Suggestions == nil {
		t.Errorf("expected non-nil suggestions slice")
	}
}

func TestRenderPrompt(t *testing.T) {
	template := "你好，{{name}}！欢迎来到{{place}}。"
	variables := map[string]string{
		"name":  "张三",
		"place": "Go世界",
	}
	result := RenderPrompt(template, variables)
	expected := "你好，张三！欢迎来到Go世界。"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}
