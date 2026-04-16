// Package mock 提供AI能力的Mock实现
// factory.go 提供创建Mock AI客户端的工厂函数
package mock

import (
	"makejob-backend/internal/ai"
)

// ProviderType AI Provider类型
type ProviderType string

const (
	// ProviderTypeMock Mock Provider（当前默认）
	ProviderTypeMock ProviderType = "mock"
	// ProviderTypeOpenAI OpenAI Provider（预留）
	ProviderTypeOpenAI ProviderType = "openai"
	// ProviderTypeAzure Azure OpenAI Provider（预留）
	ProviderTypeAzure ProviderType = "azure"
	// ProviderTypeEino Eino框架Provider（预留）
	ProviderTypeEino ProviderType = "eino"
)

// NewAIProvider 创建AI Provider实例
// providerType: Provider类型（mock/openai/azure/eino）
// config: Provider配置参数
// 返回: AIProvider接口实例
//
// 当前阶段仅支持Mock实现，后续将支持真实AI Provider
func NewAIProvider(providerType string, config map[string]string) ai.AIProvider {
	switch ProviderType(providerType) {
	case ProviderTypeMock, "":
		modelName := "mock-llm-v1"
		if config != nil {
			if name, ok := config["model_name"]; ok {
				modelName = name
			}
		}
		return NewMockProvider(modelName)
	case ProviderTypeOpenAI:
		// TODO: 实现OpenAI Provider
		return NewMockProvider("openai-mock")
	case ProviderTypeAzure:
		// TODO: 实现Azure OpenAI Provider
		return NewMockProvider("azure-mock")
	case ProviderTypeEino:
		// TODO: 实现Eino框架集成
		return NewMockProvider("eino-mock")
	default:
		// 默认返回Mock实现
		return NewMockProvider("default-mock")
	}
}

// NewInterviewAgent 创建面试Agent实例
// provider: AI Provider实例
// 返回: InterviewAgent接口实例
func NewInterviewAgent(provider ai.AIProvider) ai.InterviewAgent {
	if mockProvider, ok := provider.(*MockProvider); ok {
		return NewMockInterviewAgent(mockProvider)
	}
	return NewMockInterviewAgent(NewMockProvider("interview-mock"))
}

// NewPlanAgent 创建学习规划Agent实例
// provider: AI Provider实例
// 返回: PlanAgent接口实例
func NewPlanAgent(provider ai.AIProvider) ai.PlanAgent {
	if mockProvider, ok := provider.(*MockProvider); ok {
		return NewMockPlanAgent(mockProvider)
	}
	return NewMockPlanAgent(NewMockProvider("plan-mock"))
}

// NewCompanionAgent 创建陪伴聊天Agent实例
// provider: AI Provider实例
// 返回: CompanionAgent接口实例
func NewCompanionAgent(provider ai.AIProvider) ai.CompanionAgent {
	if mockProvider, ok := provider.(*MockProvider); ok {
		return NewMockCompanionAgent(mockProvider)
	}
	return NewMockCompanionAgent(NewMockProvider("companion-mock"))
}

// NewQuizAnalyzer 创建刷题分析Agent实例
// provider: AI Provider实例
// 返回: QuizAnalyzer接口实例
func NewQuizAnalyzer(provider ai.AIProvider) ai.QuizAnalyzer {
	if mockProvider, ok := provider.(*MockProvider); ok {
		return NewMockQuizAnalyzer(mockProvider)
	}
	return NewMockQuizAnalyzer(NewMockProvider("quiz-mock"))
}

// NewAIClient 创建AI客户端
// providerType: Provider类型
// config: Provider配置
// 返回: AIClient实例
func NewAIClient(providerType string, config map[string]string) *ai.AIClient {
	provider := NewAIProvider(providerType, config)

	return &ai.AIClient{
		Provider:       provider,
		InterviewAgent: NewInterviewAgent(provider),
		PlanAgent:      NewPlanAgent(provider),
		CompanionAgent: NewCompanionAgent(provider),
		QuizAnalyzer:   NewQuizAnalyzer(provider),
	}
}
