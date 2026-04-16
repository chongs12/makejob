// Package ai 提供AI能力抽象层
// factory.go 提供AI组件的工厂函数，用于创建各种AI Agent实例
//
// 注意：工厂函数返回接口类型，具体实现在mock子包中
// 使用示例：
//
//	client := ai.NewAIClient("mock", nil)
//	sessionID, question, err := client.InterviewAgent.StartInterview(ctx, config)
package ai

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

// AIClient AI客户端封装
// 提供统一的AI能力访问入口
type AIClient struct {
	Provider       AIProvider
	InterviewAgent InterviewAgent
	PlanAgent      PlanAgent
	CompanionAgent CompanionAgent
	QuizAnalyzer   QuizAnalyzer
}

// GetModelName 获取当前使用的模型名称
func (c *AIClient) GetModelName() string {
	if c.Provider != nil {
		return c.Provider.GetModelName()
	}
	return "unknown"
}
