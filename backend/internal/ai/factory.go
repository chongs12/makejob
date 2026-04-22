// Package ai 提供AI能力抽象层
// factory.go 定义 AI Provider 类型与统一客户端结构，供运行时装配复用。
package ai

// ProviderType AI Provider类型
type ProviderType string

const (
	// ProviderTypeMock 历史遗留的 Mock Provider 标识，运行时已不再支持。
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
