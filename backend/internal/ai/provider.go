package ai

import "context"

// ChatResponse 包含 AI 模型的回复内容和 token 用量信息。
type ChatResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
}

// TokenUsage 记录一次 AI 调用的 token 消耗。
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

// AIProvider 基础AI Provider接口
// 定义了与AI模型交互的基础能力，后续可通过Eino框架实现
type AIProvider interface {
	// Chat 发送对话请求，返回AI回复及token用量
	// messages: 对话历史消息列表
	// 返回: AI生成的回复（含token用量），或错误
	Chat(ctx context.Context, messages []Message) (*ChatResponse, error)

	// StreamChat 发送流式对话请求，返回流式响应通道
	// messages: 对话历史消息列表
	// 返回: 流式响应通道，或错误
	StreamChat(ctx context.Context, messages []Message) (<-chan string, error)

	// GetModelName 获取当前使用的模型名称
	GetModelName() string
}
