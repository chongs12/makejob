package data

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"makejob/app/ai_gateway/internal/biz"
)

// newTestPromptRepo 创建内存版 Prompt 仓库，用于验证场景别名回退。
func newTestPromptRepo(t *testing.T) *promptRepo {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&biz.PromptTemplate{}); err != nil {
		t.Fatalf("failed to migrate prompt_templates: %v", err)
	}
	return &promptRepo{db: db}
}

// TestGetActiveTemplateFallsBackToLegacyQuizScene 验证 question_generator 会回退读取历史 quiz 模板。
func TestGetActiveTemplateFallsBackToLegacyQuizScene(t *testing.T) {
	repo := newTestPromptRepo(t)
	ctx := context.Background()

	if err := repo.db.WithContext(ctx).Create(&biz.PromptTemplate{
		Scene:        "quiz",
		Version:      2,
		TemplateText: "quiz template",
		IsActive:     true,
	}).Error; err != nil {
		t.Fatalf("failed to seed prompt template: %v", err)
	}

	tpl, err := repo.GetActiveTemplate(ctx, "question_generator")
	if err != nil {
		t.Fatalf("GetActiveTemplate returned error: %v", err)
	}
	if tpl.TemplateText != "quiz template" {
		t.Fatalf("expected legacy quiz template, got %q", tpl.TemplateText)
	}
}

// TestPromptSceneCandidates 验证微服务场景名会映射到单体时代的模板场景名。
func TestPromptSceneCandidates(t *testing.T) {
	candidates := promptSceneCandidates("interview_agent")
	if len(candidates) != 2 || candidates[0] != "interview_agent" || candidates[1] != "interview" {
		t.Fatalf("unexpected interview candidates: %+v", candidates)
	}
}
