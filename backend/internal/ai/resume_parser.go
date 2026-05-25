package ai

import "context"

// ResumeParser 定义从简历文本中提取结构化画像的能力。
type ResumeParser interface {
	// Parse 从原始简历文本和可选的岗位描述中提取结构化候选人画像。
	Parse(ctx context.Context, resumeText string, jobDescription string) (*ResumeProfile, error)
}
