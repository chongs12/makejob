package biz

import "strings"

// DetectQuestionLanguage 根据题目信息推断默认编程语言
func DetectQuestionLanguage(question *Question) string {
	if question == nil {
		return "text"
	}

	content := strings.ToLower(strings.Join([]string{
		question.Title,
		question.Content,
		strings.Join(question.Tags, " "),
		question.Answer,
	}, " "))

	switch {
	case strings.Contains(content, "golang") || strings.Contains(content, " go "):
		return "go"
	case strings.Contains(content, "java"):
		return "java"
	case strings.Contains(content, "python"):
		return "python"
	case strings.Contains(content, "javascript") || strings.Contains(content, "typescript") || strings.Contains(content, "node"):
		return "javascript"
	case strings.Contains(content, "c++") || strings.Contains(content, "cpp"):
		return "cpp"
	case strings.Contains(content, "sql"):
		return "sql"
	case question.Type == "subjective":
		return "text"
	default:
		return "go"
	}
}

// NormalizeLearningPhase 标准化学习阶段枚举，未知值回退到基础阶段
func NormalizeLearningPhase(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case LearningPhaseFoundation, LearningPhaseDrill, LearningPhaseReview, LearningPhaseMock:
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return LearningPhaseFoundation
	}
}

// BuildLearningPhaseGoal 返回阶段对应的默认阶段目标文案
func BuildLearningPhaseGoal(phase string) string {
	switch NormalizeLearningPhase(phase) {
	case LearningPhaseDrill:
		return "围绕当前高频薄弱点做专项强化训练。"
	case LearningPhaseReview:
		return "回看近期训练表现，修正易错点并巩固方法。"
	case LearningPhaseMock:
		return "用模拟或限时任务验证当前阶段的真实掌握度。"
	default:
		return "先补齐核心概念、基础方法和通用解题框架。"
	}
}
