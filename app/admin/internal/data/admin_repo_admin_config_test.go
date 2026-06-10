package data

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"makejob/app/admin/internal/data/model"
)

// TestAdminConfigRowWithoutUnique 模拟历史 admin_configs 表未声明唯一约束的结构。
type TestAdminConfigRowWithoutUnique struct {
	Key         string
	Value       string
	ConfigType  string
	Description string
}

// TableName 返回历史后台配置表表名，确保测试命中真实仓库 SQL。
func (TestAdminConfigRowWithoutUnique) TableName() string { return "admin_configs" }

// newAdminConfigCompatRepo 创建仅包含 admin_configs 兼容表结构的仓库，验证写配置不会依赖唯一约束。
func newAdminConfigCompatRepo(t *testing.T) *adminRepo {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.Migrator().CreateTable(&TestAdminConfigRowWithoutUnique{}); err != nil {
		t.Fatalf("failed to create legacy admin_configs table: %v", err)
	}
	return NewAdminRepo(db).(*adminRepo)
}

// TestSetAdminConfigWithoutUniqueConstraint 验证单条配置写入在历史表结构下也能更新成功。
func TestSetAdminConfigWithoutUniqueConstraint(t *testing.T) {
	repo := newAdminConfigCompatRepo(t)
	ctx := context.Background()

	if err := repo.db.WithContext(ctx).Table("admin_configs").Create(map[string]interface{}{
		"key":   "ai_model",
		"value": "old-model",
	}).Error; err != nil {
		t.Fatalf("failed to seed config: %v", err)
	}

	if err := repo.SetAdminConfig(ctx, "ai_model", "new-model"); err != nil {
		t.Fatalf("SetAdminConfig returned error: %v", err)
	}

	value, err := repo.GetAdminConfig(ctx, "ai_model")
	if err != nil {
		t.Fatalf("GetAdminConfig returned error: %v", err)
	}
	if value != "new-model" {
		t.Fatalf("expected updated value new-model, got %q", value)
	}
}

// TestBatchUpsertConfigsWithoutUniqueConstraint 验证批量写配置在历史表结构下仍可正常插入和更新。
func TestBatchUpsertConfigsWithoutUniqueConstraint(t *testing.T) {
	repo := newAdminConfigCompatRepo(t)
	ctx := context.Background()

	if err := repo.db.WithContext(ctx).Table("admin_configs").Create(map[string]interface{}{
		"key":   "ai_provider",
		"value": "eino",
	}).Error; err != nil {
		t.Fatalf("failed to seed config: %v", err)
	}

	if err := repo.BatchUpsertConfigs(ctx, map[string]string{
		"ai_provider": "eino",
		"ai_model":    "gpt-4o-mini",
	}); err != nil {
		t.Fatalf("BatchUpsertConfigs returned error: %v", err)
	}

	provider, err := repo.GetAdminConfig(ctx, "ai_provider")
	if err != nil {
		t.Fatalf("GetAdminConfig provider returned error: %v", err)
	}
	if provider != "eino" {
		t.Fatalf("expected provider eino, got %q", provider)
	}

	model, err := repo.GetAdminConfig(ctx, "ai_model")
	if err != nil {
		t.Fatalf("GetAdminConfig model returned error: %v", err)
	}
	if model != "gpt-4o-mini" {
		t.Fatalf("expected model gpt-4o-mini, got %q", model)
	}
}

// newAdminConfigTimestampCompatRepo 创建仅包含 key/value/timestamps 的历史表结构，验证写入不会依赖额外列。
func newAdminConfigTimestampCompatRepo(t *testing.T) *adminRepo {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE admin_configs (
			key TEXT PRIMARY KEY,
			value TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`).Error; err != nil {
		t.Fatalf("failed to create timestamp compat admin_configs table: %v", err)
	}
	return NewAdminRepo(db).(*adminRepo)
}

// TestBatchUpsertConfigsWithTimestampColumns 验证兼容写入会自动补齐历史表要求的时间戳字段。
func TestBatchUpsertConfigsWithTimestampColumns(t *testing.T) {
	repo := newAdminConfigTimestampCompatRepo(t)
	ctx := context.Background()
	now := time.Now().Add(-time.Hour)

	if err := repo.db.WithContext(ctx).Table("admin_configs").Create(map[string]interface{}{
		"key":        "ai_provider",
		"value":      "legacy",
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("failed to seed timestamp compat config: %v", err)
	}

	if err := repo.BatchUpsertConfigs(ctx, map[string]string{
		"ai_provider": "eino",
		"ai_model":    "gpt-4o-mini",
	}); err != nil {
		t.Fatalf("BatchUpsertConfigs returned error: %v", err)
	}

	provider, err := repo.GetAdminConfig(ctx, "ai_provider")
	if err != nil {
		t.Fatalf("GetAdminConfig provider returned error: %v", err)
	}
	if provider != "eino" {
		t.Fatalf("expected provider eino, got %q", provider)
	}

	model, err := repo.GetAdminConfig(ctx, "ai_model")
	if err != nil {
		t.Fatalf("GetAdminConfig model returned error: %v", err)
	}
	if model != "gpt-4o-mini" {
		t.Fatalf("expected model gpt-4o-mini, got %q", model)
	}
}

// newAdminConfigCurrentSchemaRepo 创建接近当前 Postgres 线上结构的配置表，验证 config_key/config_value 映射。
func newAdminConfigCurrentSchemaRepo(t *testing.T) *adminRepo {
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
	return NewAdminRepo(db).(*adminRepo)
}

// TestBatchUpsertConfigsWithCurrentSchema 验证当前线上 config_key/config_value 结构可以正常读写。
func TestBatchUpsertConfigsWithCurrentSchema(t *testing.T) {
	repo := newAdminConfigCurrentSchemaRepo(t)
	ctx := context.Background()

	if err := repo.BatchUpsertConfigs(ctx, map[string]string{
		"ai_provider": "eino",
		"ai_model":    "gpt-4o-mini",
	}); err != nil {
		t.Fatalf("BatchUpsertConfigs returned error: %v", err)
	}

	provider, err := repo.GetAdminConfig(ctx, "ai_provider")
	if err != nil {
		t.Fatalf("GetAdminConfig provider returned error: %v", err)
	}
	if provider != "eino" {
		t.Fatalf("expected provider eino, got %q", provider)
	}

	items, err := repo.ListAdminConfigs(ctx)
	if err != nil {
		t.Fatalf("ListAdminConfigs returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 config items, got %d", len(items))
	}
}

// TestDecodeAIPresetConfigsReadsConfigJSON 验证 AI 预设会从 config_json 读取当前配置快照。
func TestDecodeAIPresetConfigsReadsConfigJSON(t *testing.T) {
	configs := decodeAIPresetConfigs(model.AIPreset{
		ConfigJSON: `{"ai_provider":"eino","ai_model":"deepseek-v3","ai_temperature":"0.8"}`,
	})

	if configs["ai_provider"] != "eino" {
		t.Fatalf("expected ai_provider from config_json, got %q", configs["ai_provider"])
	}
	if configs["ai_model"] != "deepseek-v3" {
		t.Fatalf("expected ai_model from config_json, got %q", configs["ai_model"])
	}
	if configs["ai_temperature"] != "0.8" {
		t.Fatalf("expected ai_temperature from config_json, got %q", configs["ai_temperature"])
	}
}
