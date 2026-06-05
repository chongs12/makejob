package data

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"makejob/app/admin/internal/biz"
	"makejob/app/admin/internal/data/model"
)

// TestCreatePromptTemplateUsesNilIndustryID 验证通用 Prompt 模板不会被错误写成 industry_id=0。
func TestCreatePromptTemplateUsesNilIndustryID(t *testing.T) {
	db := newPromptTestDB(t)
	repo := NewAdminRepo(db).(*adminRepo)

	if err := repo.CreatePromptTemplate(context.Background(), &biz.PromptTemplate{
		Name:            "generic",
		Scene:           "quiz",
		TemplateContent: "content",
		IsActive:        true,
	}); err != nil {
		t.Fatalf("CreatePromptTemplate returned error: %v", err)
	}

	var stored model.PromptTemplate
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load prompt template: %v", err)
	}
	if stored.IndustryID != nil {
		t.Fatalf("expected nil industry_id for generic template, got %v", *stored.IndustryID)
	}
}

// TestUpdatePromptTemplateClearsIndustryID 验证更新时传入 industry_id=0 会清空行业绑定。
func TestUpdatePromptTemplateClearsIndustryID(t *testing.T) {
	db := newPromptTestDB(t)
	repo := NewAdminRepo(db).(*adminRepo)

	industryID := uint(7)
	record := &model.PromptTemplate{
		IndustryID:      &industryID,
		Name:            "bound",
		Scene:           "quiz",
		TemplateContent: "content",
		IsActive:        true,
	}
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("failed to seed prompt template: %v", err)
	}

	if err := repo.UpdatePromptTemplate(context.Background(), &biz.PromptTemplate{
		ID:              uint64(record.ID),
		IndustryID:      0,
		Name:            "bound",
		Scene:           "quiz",
		TemplateContent: "updated",
		IsActive:        true,
	}); err != nil {
		t.Fatalf("UpdatePromptTemplate returned error: %v", err)
	}

	var stored model.PromptTemplate
	if err := db.First(&stored, record.ID).Error; err != nil {
		t.Fatalf("failed to reload prompt template: %v", err)
	}
	if stored.IndustryID != nil {
		t.Fatalf("expected nil industry_id after clearing binding, got %v", *stored.IndustryID)
	}
}

// newPromptTestDB 创建 prompt 数据层回归测试使用的最小 SQLite 数据库。
func newPromptTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(&model.Industry{}, &model.PromptTemplate{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}
	return db
}
