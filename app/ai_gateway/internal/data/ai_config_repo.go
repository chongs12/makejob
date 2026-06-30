package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"makejob/app/ai_gateway/internal/biz"
)

type aiConfigRepo struct {
	db *gorm.DB
}

// adminConfigCompatProbe 仅用于探测 admin_configs 的真实列集合，兼容新旧 schema。
type adminConfigCompatProbe struct{}

// TableName 返回后台 AI 通用配置表名，供列探测逻辑复用。
func (adminConfigCompatProbe) TableName() string { return "admin_configs" }

// adminConfigRow 对应 admin_configs 的最小兼容读取结构，统一承接不同键值列的别名。
type adminConfigRow struct {
	Key   string
	Value string
}

// NewAIConfigRepo 创建 AI 配置仓库实现。
func NewAIConfigRepo(db *gorm.DB) biz.AIConfigRepo {
	return &aiConfigRepo{db: db}
}

// GetActiveConfig 查询指定场景下当前生效的 AI 配置。
func (r *aiConfigRepo) GetActiveConfig(ctx context.Context, scene string) (*biz.AIConfig, error) {
	var cfg biz.AIConfig
	err := r.db.WithContext(ctx).
		Where("scene = ? AND is_active = ?", scene, true).
		First(&cfg).Error
	if err == nil {
		return &cfg, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return r.buildConfigFromAdminConfigs(ctx, scene)
}

// buildConfigFromAdminConfigs 将后台 AI 配置页写入的 admin_configs 兜底映射为当前场景的运行时配置。
func (r *aiConfigRepo) buildConfigFromAdminConfigs(ctx context.Context, scene string) (*biz.AIConfig, error) {
	sceneModelKey := sceneModelConfigKey(scene)
	keys := []string{
		"ai_provider",
		"ai_model",
		"ai_temperature",
		"ai_max_tokens",
		"ai_top_p",
		"ai_timeout_seconds",
		"ai_enable_stream",
		"ai_api_key",
		"ai_base_url",
		"ai_fallback_provider",
	}
	if sceneModelKey != "" {
		keys = append(keys, sceneModelKey)
	}

	columns, err := r.loadAdminConfigColumns(ctx)
	if err != nil {
		return nil, err
	}
	keyColumn := aiGatewayAdminConfigKeyColumn(columns)
	valueColumn := aiGatewayAdminConfigValueColumn(columns)

	var rows []adminConfigRow
	query := r.db.WithContext(ctx).
		Table("admin_configs").
		Select(fmt.Sprintf("%s AS key, %s AS value", keyColumn, valueColumn)).
		Where(fmt.Sprintf("%s IN ?", keyColumn), keys)
	if aiGatewayHasAdminConfigDeletedAt(columns) {
		query = query.Where("deleted_at IS NULL")
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	configs := map[string]string{
		"ai_provider":        "eino",
		"ai_model":           "gpt-4o-mini",
		"ai_temperature":     "0.7",
		"ai_max_tokens":      "2048",
		"ai_top_p":           "0.9",
		"ai_timeout_seconds": "30",
		"ai_enable_stream":   "false",
	}
	for _, row := range rows {
		if strings.TrimSpace(row.Key) == "" {
			continue
		}
		configs[row.Key] = strings.TrimSpace(row.Value)
	}

	modelName := firstNonEmptyConfig(configs[sceneModelKey], configs["ai_model"])
	if modelName == "" {
		return nil, gorm.ErrRecordNotFound
	}

	extraConfigJSON, _ := json.Marshal(map[string]string{
		"ai_top_p":             strings.TrimSpace(configs["ai_top_p"]),
		"ai_timeout_seconds":   strings.TrimSpace(configs["ai_timeout_seconds"]),
		"ai_enable_stream":     strings.TrimSpace(configs["ai_enable_stream"]),
		"ai_api_key":           strings.TrimSpace(configs["ai_api_key"]),
		"ai_base_url":          strings.TrimSpace(configs["ai_base_url"]),
		"ai_fallback_provider": strings.TrimSpace(configs["ai_fallback_provider"]),
		"ai_fallback_api_key":  strings.TrimSpace(configs["ai_fallback_api_key"]),
		"ai_fallback_base_url": strings.TrimSpace(configs["ai_fallback_base_url"]),
		"ai_fallback_model":    strings.TrimSpace(configs["ai_fallback_model"]),
		"scene_model_key":      sceneModelKey,
	})

	return &biz.AIConfig{
		Scene:           scene,
		Provider:        firstNonEmptyConfig(configs["ai_provider"], "eino"),
		Model:           modelName,
		Temperature:     parseFloatConfig(configs["ai_temperature"], 0.7),
		MaxTokens:       parseIntConfig(configs["ai_max_tokens"], 2048),
		ExtraParamsJSON: string(extraConfigJSON),
		IsActive:        true,
	}, nil
}

// loadAdminConfigColumns 读取 admin_configs 的真实列集合，供兼容读取 key/value 与 config_key/config_value 两套结构。
func (r *aiConfigRepo) loadAdminConfigColumns(ctx context.Context) (map[string]struct{}, error) {
	columnTypes, err := r.db.WithContext(ctx).Migrator().ColumnTypes(&adminConfigCompatProbe{})
	if err != nil {
		return nil, err
	}
	columns := make(map[string]struct{}, len(columnTypes))
	for _, columnType := range columnTypes {
		columns[strings.ToLower(columnType.Name())] = struct{}{}
	}
	return columns, nil
}

// aiGatewayAdminConfigKeyColumn 返回当前 admin_configs 表实际使用的配置键列名。
func aiGatewayAdminConfigKeyColumn(columns map[string]struct{}) string {
	if _, ok := columns["config_key"]; ok {
		return "config_key"
	}
	return "key"
}

// aiGatewayAdminConfigValueColumn 返回当前 admin_configs 表实际使用的配置值列名。
func aiGatewayAdminConfigValueColumn(columns map[string]struct{}) string {
	if _, ok := columns["config_value"]; ok {
		return "config_value"
	}
	return "value"
}

// aiGatewayHasAdminConfigDeletedAt 判断后台配置表是否存在软删除列，避免读取到已删除配置。
func aiGatewayHasAdminConfigDeletedAt(columns map[string]struct{}) bool {
	_, ok := columns["deleted_at"]
	return ok
}

// sceneModelConfigKey 返回场景对应的模型覆写键，兼容后台 AI 配置页中的 scene 级模型设置。
func sceneModelConfigKey(scene string) string {
	switch strings.TrimSpace(scene) {
	case "interview_agent":
		return "ai_scene_interview_model"
	case "plan_agent":
		return "ai_scene_plan_model"
	case "companion_agent":
		return "ai_scene_companion_model"
	case "question_generator", "quiz_analyzer":
		return "ai_scene_quiz_model"
	default:
		return ""
	}
}

// firstNonEmptyConfig 返回第一个非空配置值，便于在场景覆写与全局默认之间回退。
func firstNonEmptyConfig(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// parseFloatConfig 解析浮点配置，失败时回退到给定默认值，避免旧配置缺字段导致运行时中断。
func parseFloatConfig(raw string, fallback float64) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return fallback
	}
	return value
}

// parseIntConfig 解析整数配置，失败时回退到给定默认值，避免旧配置缺字段导致运行时中断。
func parseIntConfig(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}
