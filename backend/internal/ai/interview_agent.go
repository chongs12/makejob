package ai

import "context"

// InterviewAgent 面试Agent接口
// 提供模拟面试的完整流程控制，包括开始面试、评估答案、生成报告等
type InterviewAgent interface {
	// StartInterview 开始一场面试
	// ctx: 上下文
	// config: 面试配置（行业、难度、题目数量等）
	// 返回: 会话ID、第一题、错误
	StartInterview(ctx context.Context, config InterviewConfig) (sessionID string, firstQuestion InterviewQuestion, err error)

	// EvaluateAnswer 评估用户答案
	// ctx: 上下文
	// sessionID: 面试会话ID
	// questionIndex: 当前题目索引
	// answer: 用户答案
	// 返回: 答案反馈、错误
	EvaluateAnswer(ctx context.Context, sessionID string, questionIndex int, answer string) (AnswerFeedback, error)

	// GetNextQuestion 获取下一道面试题
	// ctx: 上下文
	// sessionID: 面试会话ID
	// 返回: 下一道题目、是否还有下一题、错误
	GetNextQuestion(ctx context.Context, sessionID string) (InterviewQuestion, bool, error)

	// GenerateReport 生成面试报告
	// ctx: 上下文
	// sessionID: 面试会话ID
	// 返回: 面试报告、错误
	GenerateReport(ctx context.Context, sessionID string) (InterviewReport, error)

	// EndInterview 结束面试会话
	// ctx: 上下文
	// sessionID: 面试会话ID
	// 返回: 错误
	EndInterview(ctx context.Context, sessionID string) error
}
