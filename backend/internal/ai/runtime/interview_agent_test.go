package runtime

import (
	"testing"

	"makejob-backend/internal/ai"
)

// TestDecodeJSONPayloadFromMarkdown 验证模型返回带代码块时仍能提取 JSON。
func TestDecodeJSONPayloadFromMarkdown(t *testing.T) {
	raw := "```json\n{\"question\":\"解释 goroutine\",\"topic\":\"并发编程\",\"difficulty\":\"medium\",\"type\":\"technical\",\"hints\":\"说明调度模型\"}\n```"

	payload, err := decodeJSONPayload[interviewQuestionPayload](raw)
	if err != nil {
		t.Fatalf("decodeJSONPayload returned error: %v", err)
	}

	if payload.Question != "解释 goroutine" {
		t.Fatalf("expected question to be decoded, got %q", payload.Question)
	}
}

// TestNormalizeQuestionPayloadFallsBackMissingFields 验证缺失字段会按会话配置补齐。
func TestNormalizeQuestionPayloadFallsBackMissingFields(t *testing.T) {
	session := &interviewSessionState{
		Config: ai.InterviewConfig{
			IndustryCode:  "go",
			Difficulty:    "mixed",
			Topics:        []string{"并发编程"},
			QuestionCount: 3,
		},
	}

	question, err := normalizeQuestionPayload(interviewQuestionPayload{
		Question: "请解释 goroutine 和线程的区别。",
	}, session, 0)
	if err != nil {
		t.Fatalf("normalizeQuestionPayload returned error: %v", err)
	}

	if question.Topic != "并发编程" {
		t.Fatalf("expected fallback topic, got %q", question.Topic)
	}
	if question.Difficulty != "easy" {
		t.Fatalf("expected first mixed question difficulty easy, got %q", question.Difficulty)
	}
	if question.Type != "technical" {
		t.Fatalf("expected default type technical, got %q", question.Type)
	}
}

// TestBuildLocalReportAggregatesFeedback 验证本地报告聚合逻辑可输出有效统计。
func TestBuildLocalReportAggregatesFeedback(t *testing.T) {
	session := &interviewSessionState{
		Questions: []ai.InterviewQuestion{
			{Question: "Q1", Topic: "并发编程"},
			{Question: "Q2", Topic: "基础语法"},
		},
		Answers: []string{"answer-1", "answer-2"},
		Feedbacks: []ai.AnswerFeedback{
			{Score: 80, IsCorrect: true},
			{Score: 60, IsCorrect: true},
		},
	}

	report := buildLocalReport(session)

	if report.OverallScore != 70 {
		t.Fatalf("expected overall score 70, got %v", report.OverallScore)
	}
	if report.CorrectCount != 2 {
		t.Fatalf("expected correct count 2, got %d", report.CorrectCount)
	}
	if len(report.DimensionScores) != 2 {
		t.Fatalf("expected 2 dimension scores, got %d", len(report.DimensionScores))
	}
}
