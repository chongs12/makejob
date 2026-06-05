package biz

import (
	"context"
	"strings"
)

// Message LLM 对话消息
type Message struct {
	Role    string
	Content string
}

// LLMResponse LLM 调用返回结果
type LLMResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
}

// LLMClient 大模型调用客户端接口
type LLMClient interface {
	// Chat 发送对话请求并获取响应
	Chat(ctx context.Context, messages []Message, config *AIConfig) (*LLMResponse, error)
}

// RenderPrompt 使用变量替换渲染 Prompt 模板文本
func RenderPrompt(templateText string, variables map[string]string) string {
	result := templateText
	for k, v := range variables {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}
