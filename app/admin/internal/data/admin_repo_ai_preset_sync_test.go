package data

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"makejob/app/admin/internal/data/model"
)

// newAIConfigSyncRepo 构造同时包含 admin_configs（当前 schema）与 ai_presets 表的内存仓库，
// 供“保存 AI 配置时同步激活预设快照”的用例验证。
func newAIConfigSyncRepo(t *testing.T) *adminRepo {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE admin_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			deleted_at DATETIME NULL,
			config_key TEXT NOT NULL,
			config_value TEXT NOT NULL,
			config_type TEXT NOT NULL DEFAULT 'string',
			description TEXT NULL
		)
	`).Error; err != nil {
		t.Fatalf("failed to create current schema admin_configs table: %v", err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX idx_admin_configs_config_key ON admin_configs(config_key)`).Error; err != nil {
		t.Fatalf("failed to create config_key index: %v", err)
	}
	if err := db.Migrator().CreateTable(&model.AIPreset{}); err != nil {
		t.Fatalf("failed to create ai_presets table: %v", err)
	}
	return NewAdminRepo(db).(*adminRepo)
}

// TestBatchUpsertConfigsSyncActivePresetMergesSnapshot 验证保存 AI 配置时，
// 激活预设快照会合并新值且保留表单未覆盖的字段，非激活预设不受影响。
func TestBatchUpsertConfigsSyncActivePresetMergesSnapshot(t *testing.T) {
	repo := newAIConfigSyncRepo(t)
	ctx := context.Background()

	// 激活预设“字节”：快照同时含表单字段(ai_model)与非表单字段(ai_rag_collection)。
	if err := repo.db.WithContext(ctx).Create(&model.AIPreset{
		Name:       "字节",
		ConfigJSON: `{"ai_model":"doubao-old","ai_rag_collection":"interview_questions","ai_temperature":"0.8"}`,
		IsActive:   true,
	}).Error; err != nil {
		t.Fatalf("failed to seed active preset: %v", err)
	}
	// 非激活预设，验证不会被误改。
	if err := repo.db.WithContext(ctx).Create(&model.AIPreset{
		Name:       "小米",
		ConfigJSON: `{"ai_model":"mimo-old"}`,
		IsActive:   false,
	}).Error; err != nil {
		t.Fatalf("failed to seed inactive preset: %v", err)
	}

	if err := repo.BatchUpsertConfigsSyncActivePreset(ctx, map[string]string{
		"ai_model": "doubao-new",
	}); err != nil {
		t.Fatalf("BatchUpsertConfigsSyncActivePreset returned error: %v", err)
	}

	// 运行时已写入新值。
	got, err := repo.GetAdminConfig(ctx, "ai_model")
	if err != nil {
		t.Fatalf("GetAdminConfig returned error: %v", err)
	}
	if got != "doubao-new" {
		t.Fatalf("runtime ai_model = %q, want doubao-new", got)
	}

	// 激活预设快照已合并：新值写入，非表单字段保留。
	var active model.AIPreset
	if err := repo.db.WithContext(ctx).Where("name = ?", "字节").First(&active).Error; err != nil {
		t.Fatalf("failed to load active preset: %v", err)
	}
	snap := decodeAIPresetConfigs(active)
	if snap["ai_model"] != "doubao-new" {
		t.Fatalf("preset ai_model = %q, want doubao-new", snap["ai_model"])
	}
	if snap["ai_rag_collection"] != "interview_questions" {
		t.Fatalf("preset ai_rag_collection should be preserved, got %q", snap["ai_rag_collection"])
	}
	if snap["ai_temperature"] != "0.8" {
		t.Fatalf("preset ai_temperature should be preserved, got %q", snap["ai_temperature"])
	}

	// 非激活预设快照不变。
	var inactive model.AIPreset
	if err := repo.db.WithContext(ctx).Where("name = ?", "小米").First(&inactive).Error; err != nil {
		t.Fatalf("failed to load inactive preset: %v", err)
	}
	if decodeAIPresetConfigs(inactive)["ai_model"] != "mimo-old" {
		t.Fatalf("inactive preset snapshot should not be touched, got %q", decodeAIPresetConfigs(inactive)["ai_model"])
	}
}

// TestBatchUpsertConfigsSyncActivePresetWithoutActiveIsNoop 验证没有激活预设时，
// 同步逻辑静默跳过，但运行时配置仍正常写入。
func TestBatchUpsertConfigsSyncActivePresetWithoutActiveIsNoop(t *testing.T) {
	repo := newAIConfigSyncRepo(t)
	ctx := context.Background()

	if err := repo.BatchUpsertConfigsSyncActivePreset(ctx, map[string]string{
		"ai_model": "gpt-4o-mini",
	}); err != nil {
		t.Fatalf("should not error without active preset: %v", err)
	}

	got, err := repo.GetAdminConfig(ctx, "ai_model")
	if err != nil {
		t.Fatalf("GetAdminConfig returned error: %v", err)
	}
	if got != "gpt-4o-mini" {
		t.Fatalf("runtime ai_model = %q, want gpt-4o-mini", got)
	}

	var count int64
	if err := repo.db.WithContext(ctx).Model(&model.AIPreset{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count presets: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no preset rows, got %d", count)
	}
}
