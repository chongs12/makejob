// Package mock 提供AI能力的Mock实现
// 用于开发和测试阶段，后续可无缝替换为真实AI Provider
package mock

import (
	"context"
	"fmt"
	"time"

	"makejob-backend/internal/ai"
)

// MockProvider Mock AI Provider实现
type MockProvider struct {
	modelName string
}

// NewMockProvider 创建Mock Provider实例
func NewMockProvider(modelName string) *MockProvider {
	if modelName == "" {
		modelName = "mock-llm-v1"
	}
	return &MockProvider{
		modelName: modelName,
	}
}

// Chat 模拟对话请求
func (p *MockProvider) Chat(ctx context.Context, messages []ai.Message) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(100 * time.Millisecond): // 模拟网络延迟
		if len(messages) == 0 {
			return "", fmt.Errorf("messages cannot be empty")
		}
		lastMsg := messages[len(messages)-1]
		return p.generateMockResponse(lastMsg.Content), nil
	}
}

// StreamChat 模拟流式对话请求
func (p *MockProvider) StreamChat(ctx context.Context, messages []ai.Message) (<-chan string, error) {
	responseChan := make(chan string)

	go func() {
		defer close(responseChan)

		mockResponse := "这是一个Mock流式响应。实际集成后会连接真实的AI模型服务。"
		if len(messages) > 0 {
			mockResponse = p.generateMockResponse(messages[len(messages)-1].Content)
		}

		// 模拟流式输出
		for _, char := range mockResponse {
			select {
			case <-ctx.Done():
				return
			case responseChan <- string(char):
				time.Sleep(20 * time.Millisecond) // 模拟打字效果
			}
		}
	}()

	return responseChan, nil
}

// GetModelName 获取模型名称
func (p *MockProvider) GetModelName() string {
	return p.modelName
}

// generateMockResponse 生成Mock响应
func (p *MockProvider) generateMockResponse(userInput string) string {
	// 根据输入内容返回不同的预设回复
	responses := map[string]string{
		"你好":    "你好！我是你的AI助手，很高兴为你服务。",
		"hello": "Hello! I'm your AI assistant. How can I help you today?",
		"帮助":    "我可以帮助你解答问题、分析代码、制定学习计划等。请告诉我你需要什么帮助。",
		"面试":    "我可以帮你进行模拟面试。请告诉我你想面试的技术方向和难度级别。",
		"计划":    "我可以为你制定个性化的学习计划。请告诉我你的学习目标和当前水平。",
	}

	for keyword, response := range responses {
		if contains(userInput, keyword) {
			return response
		}
	}

	return "我理解你的问题。作为一个Mock AI，我会返回预设的回复。实际集成后将连接真实的AI模型提供更智能的回复。"
}

// contains 检查字符串是否包含关键词
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
