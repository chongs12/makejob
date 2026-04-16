package ai

import "context"

// AIProvider 基础AI Provider接口
// 定义了与AI模型交互的基础能力，后续可通过Eino框架实现
type AIProvider interface {
	// Chat 发送对话请求，返回AI回复
	// messages: 对话历史消息列表
	// 返回: AI生成的回复文本，或错误
	Chat(ctx context.Context, messages []Message) (string, error)

	// StreamChat 发送流式对话请求，返回流式响应通道
	// messages: 对话历史消息列表
	// 返回: 流式响应通道，或错误
	StreamChat(ctx context.Context, messages []Message) (<-chan string, error)

	// GetModelName 获取当前使用的模型名称
	GetModelName() string
}
