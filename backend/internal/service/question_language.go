package service

import (
	"strings"

	"makejob-backend/internal/model"
)

// detectQuestionLanguage 根据题目信息推断默认编程语言。
func detectQuestionLanguage(question *model.Question) string {
	if question == nil {
		return "text"
	}

	content := strings.ToLower(strings.Join([]string{
		question.Title,
		question.Content,
		question.Tags,
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
	case question.Type == model.QuestionTypeSubjective:
		return "text"
	default:
		return "go"
	}
}
