package ai

import "context"

// CompanionAgent 陪伴聊天Agent接口
// 提供情感陪伴、鼓励、问候等交互能力
type CompanionAgent interface {
	// Chat 进行陪伴对话
	// ctx: 上下文
	// messages: 对话历史
	// userEmotion: 用户当前情绪状态
	// 返回: 陪伴响应（包含内容、情绪、动作）、错误
	Chat(ctx context.Context, messages []Message, userEmotion string) (CompanionResponse, error)

	// GetGreeting 获取问候语
	// ctx: 上下文
	// profile: 用户画像
	// timeOfDay: 时间段（morning/afternoon/evening/night）
	// 返回: 问候响应、错误
	GetGreeting(ctx context.Context, profile UserProfile, timeOfDay string) (CompanionResponse, error)

	// GetEncouragement 获取鼓励语
	// ctx: 上下文
	// achievement: 用户成就/进度描述
	// 返回: 鼓励响应、错误
	GetEncouragement(ctx context.Context, achievement string) (CompanionResponse, error)
}
