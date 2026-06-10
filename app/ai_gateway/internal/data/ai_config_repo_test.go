package data

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"makejob/app/ai_gateway/internal/biz"
)

// testAdminConfigRow 提供 admin_configs 测试表结构，验证 AI Gateway 对后台通用配置的兼容读取。
type testAdminConfigRow struct {
	Key   string `gorm:"primaryKey;size:100"`
	Value string
}

// TableName 返回测试使用的后台配置表名，确保仓库逻辑命中真实 SQL 表名。
func (testAdminConfigRow) TableName() string { return "admin_configs" }

// newTestAIConfigRepo 创建带内存数据库的仓库实例，供配置回退逻辑测试复用。
func newTestAIConfigRepo(t *testing.T) *aiConfigRepo {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&biz.AIConfig{}, &testAdminConfigRow{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}
	return &aiConfigRepo{db: db}
}

// TestGetActiveConfigFallsBackToAdminConfigs 验证 scene 级 ai_configs 缺失时会回退读取后台 AI 配置页保存的通用配置。
func TestGetActiveConfigFallsBackToAdminConfigs(t *testing.T) {
	repo := newTestAIConfigRepo(t)
	ctx := context.Background()

	rows := []testAdminConfigRow{
		{Key: "ai_provider", Value: "eino"},
		{Key: "ai_model", Value: "global-model"},
		{Key: "ai_scene_quiz_model", Value: "quiz-scene-model"},
		{Key: "ai_temperature", Value: "0.35"},
		{Key: "ai_max_tokens", Value: "4096"},
	}
	if err := repo.db.WithContext(ctx).Create(&rows).Error; err != nil {
		t.Fatalf("failed to seed admin configs: %v", err)
	}

	cfg, err := repo.GetActiveConfig(ctx, "question_generator")
	if err != nil {
		t.Fatalf("GetActiveConfig returned error: %v", err)
	}
	if cfg.Scene != "question_generator" {
		t.Fatalf("expected scene question_generator, got %q", cfg.Scene)
	}
	if cfg.Model != "quiz-scene-model" {
		t.Fatalf("expected scene override model, got %q", cfg.Model)
	}
	if cfg.Provider != "eino" {
		t.Fatalf("expected provider eino, got %q", cfg.Provider)
	}
	if cfg.Temperature != 0.35 {
		t.Fatalf("expected temperature 0.35, got %v", cfg.Temperature)
	}
	if cfg.MaxTokens != 4096 {
		t.Fatalf("expected max tokens 4096, got %d", cfg.MaxTokens)
	}
}

// TestGetActiveConfigPrefersSceneTable 验证 ai_configs 表已有生效场景配置时，会优先使用原生 scene 级配置。
func TestGetActiveConfigPrefersSceneTable(t *testing.T) {
	repo := newTestAIConfigRepo(t)
	ctx := context.Background()

	if err := repo.db.WithContext(ctx).Create(&biz.AIConfig{
		Scene:       "question_generator",
		Provider:    "eino",
		Model:       "scene-table-model",
		Temperature: 0.2,
		MaxTokens:   1024,
		IsActive:    true,
	}).Error; err != nil {
		t.Fatalf("failed to seed ai_configs: %v", err)
	}
	if err := repo.db.WithContext(ctx).Create(&testAdminConfigRow{Key: "ai_scene_quiz_model", Value: "fallback-model"}).Error; err != nil {
		t.Fatalf("failed to seed admin config fallback: %v", err)
	}

	cfg, err := repo.GetActiveConfig(ctx, "question_generator")
	if err != nil {
		t.Fatalf("GetActiveConfig returned error: %v", err)
	}
	if cfg.Model != "scene-table-model" {
		t.Fatalf("expected native scene config to win, got %q", cfg.Model)
	}
	if cfg.MaxTokens != 1024 {
		t.Fatalf("expected native scene max tokens 1024, got %d", cfg.MaxTokens)
	}
}

// newCurrentSchemaAIConfigRepo 创建接近当前 Postgres 线上结构的配置表，验证 config_key/config_value 兼容读取。
func newCurrentSchemaAIConfigRepo(t *testing.T) *aiConfigRepo {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&biz.AIConfig{}); err != nil {
		t.Fatalf("failed to migrate ai_configs table: %v", err)
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
		t.Fatalf("failed to create current schema admin_configs index: %v", err)
	}
	return &aiConfigRepo{db: db}
}

// TestGetActiveConfigFallsBackToCurrentAdminSchema 验证 AI Gateway 能从 config_key/config_value 结构读取后台 AI 配置。
func TestGetActiveConfigFallsBackToCurrentAdminSchema(t *testing.T) {
	repo := newCurrentSchemaAIConfigRepo(t)
	ctx := context.Background()

	nowExpr := "CURRENT_TIMESTAMP"
	if err := repo.db.WithContext(ctx).Exec(`
		INSERT INTO admin_configs (created_at, updated_at, config_key, config_value, config_type, description)
		VALUES
			(` + nowExpr + `, ` + nowExpr + `, 'ai_provider', 'eino', 'string', ''),
			(` + nowExpr + `, ` + nowExpr + `, 'ai_model', 'global-model', 'string', ''),
			(` + nowExpr + `, ` + nowExpr + `, 'ai_scene_quiz_model', 'quiz-scene-model', 'string', ''),
			(` + nowExpr + `, ` + nowExpr + `, 'ai_temperature', '0.35', 'string', ''),
			(` + nowExpr + `, ` + nowExpr + `, 'ai_max_tokens', '4096', 'string', '')
	`).Error; err != nil {
		t.Fatalf("failed to seed current schema admin configs: %v", err)
	}

	cfg, err := repo.GetActiveConfig(ctx, "question_generator")
	if err != nil {
		t.Fatalf("GetActiveConfig returned error: %v", err)
	}
	if cfg.Model != "quiz-scene-model" {
		t.Fatalf("expected scene override model, got %q", cfg.Model)
	}
	if cfg.Provider != "eino" {
		t.Fatalf("expected provider eino, got %q", cfg.Provider)
	}
}
