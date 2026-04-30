package service

import (
	"testing"

	"makejob-backend/internal/model"
)

// TestBuildQuestionSetSummariesUsesFullCount 验证题单摘要中的数量使用完整命中数，而不是仅统计预览条数。
func TestBuildQuestionSetSummariesUsesFullCount(t *testing.T) {
	t.Parallel()

	questions := []model.Question{
		{BaseModel: model.BaseModel{ID: 1}, Title: "slice 扩容机制", Difficulty: "easy", Type: model.QuestionTypeSubjective},
		{BaseModel: model.BaseModel{ID: 2}, Title: "map 并发安全", Difficulty: "medium", Type: model.QuestionTypeSubjective},
		{BaseModel: model.BaseModel{ID: 3}, Title: "interface 底层表示", Difficulty: "medium", Type: model.QuestionTypeSubjective},
		{BaseModel: model.BaseModel{ID: 4}, Title: "goroutine 调度", Difficulty: "hard", Type: model.QuestionTypeSubjective},
		{BaseModel: model.BaseModel{ID: 5}, Title: "defer 执行顺序", Difficulty: "easy", Type: model.QuestionTypeSubjective},
	}

	summaries := buildQuestionSetSummaries(questions)
	if len(summaries) == 0 {
		t.Fatal("expected question set summaries, got none")
	}

	var runtimeSummary *QuestionSetSummary
	for index := range summaries {
		if summaries[index].Slug == "go-runtime-core" {
			runtimeSummary = &summaries[index]
			break
		}
	}
	if runtimeSummary == nil {
		t.Fatal("expected go-runtime-core summary, got nil")
	}
	if runtimeSummary.QuestionCount != 5 {
		t.Fatalf("expected full count 5, got %d", runtimeSummary.QuestionCount)
	}
	if len(runtimeSummary.Questions) != 4 {
		t.Fatalf("expected preview size 4, got %d", len(runtimeSummary.Questions))
	}
}

// TestBuildQuestionSetDetailReturnsMatchedQuestions 验证题单详情会返回全部命中题目，供前端稳定承接补练集合。
func TestBuildQuestionSetDetailReturnsMatchedQuestions(t *testing.T) {
	t.Parallel()

	definition, ok := findQuestionSetDefinition("go-concurrency-debug")
	if !ok {
		t.Fatal("expected predefined question set definition")
	}

	questions := []model.Question{
		{BaseModel: model.BaseModel{ID: 1}, Title: "channel 关闭规则", Difficulty: "easy", Type: model.QuestionTypeSubjective},
		{BaseModel: model.BaseModel{ID: 2}, Title: "select 随机性", Difficulty: "medium", Type: model.QuestionTypeSubjective},
		{BaseModel: model.BaseModel{ID: 3}, Title: "Gin 中间件链路", Difficulty: "medium", Type: model.QuestionTypeSubjective},
	}

	detail := buildQuestionSetDetail(definition, questions)
	if detail == nil {
		t.Fatal("expected detail, got nil")
	}
	if detail.QuestionCount != 2 {
		t.Fatalf("expected 2 matched questions, got %d", detail.QuestionCount)
	}
	if len(detail.Questions) != 2 {
		t.Fatalf("expected 2 question previews, got %d", len(detail.Questions))
	}
	if detail.Questions[0].ID != 1 || detail.Questions[1].ID != 2 {
		t.Fatalf("unexpected matched question ids: %+v", detail.Questions)
	}
}
