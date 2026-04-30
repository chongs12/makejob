package ai

import "context"

// QuizAnalyzer 刷题分析Agent接口
// 提供代码分析、答案解释、提示生成等能力
type QuizAnalyzer interface {
	// AnalyzeCode 分析用户提交的代码
	// ctx: 上下文
	// code: 用户代码
	// language: 编程语言
	// question: 题目描述
	// 返回: 代码分析结果、错误
	AnalyzeCode(ctx context.Context, code string, language string, question string) (CodeAnalysis, error)

	// DiagnoseInterviewCoding 基于题目、最终代码和过程事件生成编程面试诊断。
	// ctx: 上下文
	// input: 编程题诊断输入
	// 返回: 结构化诊断结果、错误
	DiagnoseInterviewCoding(ctx context.Context, input InterviewCodingDiagnosisInput) (CodingQuestionDiagnosis, error)

	// ExplainAnswer 解释正确答案
	// ctx: 上下文
	// questionTitle: 题目标题
	// questionContent: 题目内容
	// correctAnswer: 正确答案
	// 返回: 详细解释、错误
	ExplainAnswer(ctx context.Context, questionTitle string, questionContent string, correctAnswer string) (string, error)

	// GenerateHint 生成题目提示
	// ctx: 上下文
	// questionTitle: 题目标题
	// questionContent: 题目内容
	// 返回: 提示文本、错误
	GenerateHint(ctx context.Context, questionTitle string, questionContent string) (string, error)
}
