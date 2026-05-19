package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"makejob-backend/internal/ai"
	aiRuntime "makejob-backend/internal/ai/runtime"
	"makejob-backend/internal/model"
)

// AIConfigSupport 描述后台 AI 配置页的当前运行时支持范围。
type AIConfigSupport struct {
	PrimaryProviders  []string `json:"primary_providers"`
	FallbackProviders []string `json:"fallback_providers"`
	Notes             []string `json:"notes"`
}

// AIPresetSummary 描述单个 AI 预设的展示信息与完整配置快照。
type AIPresetSummary struct {
	ID        uint              `json:"id"`
	Name      string            `json:"name"`
	IsActive  bool              `json:"is_active"`
	UpdatedAt time.Time         `json:"updated_at"`
	Configs   map[string]string `json:"configs"`
}

// CreateAIPresetRequest 描述创建 AI 预设时需要提交的完整配置快照。
type CreateAIPresetRequest struct {
	Name    string            `json:"name"`
	Configs map[string]string `json:"configs"`
}

// UpdateAIPresetRequest 描述更新 AI 预设时允许修改的字段。
type UpdateAIPresetRequest struct {
	Name    *string           `json:"name,omitempty"`
	Configs map[string]string `json:"configs,omitempty"`
}

// AIConfigResponse 描述后台 AI 配置页需要的完整数据。
type AIConfigResponse struct {
	Configs        map[string]string   `json:"configs"`
	Items          []model.AdminConfig `json:"items"`
	Support        AIConfigSupport     `json:"support"`
	Warnings       []string            `json:"warnings"`
	Presets        []AIPresetSummary   `json:"presets"`
	ActivePresetID *uint               `json:"active_preset_id,omitempty"`
}

// buildAIConfigResponse 构建 AI 配置响应，并把配置文件默认值合并进返回结果。
func buildAIConfigResponse(items []model.AdminConfig, baseConfig map[string]string) *AIConfigResponse {
	filtered := make([]model.AdminConfig, 0, len(items))
	configMap := ai.NormalizeRuntimeConfig(baseConfig)

	for _, item := range items {
		if !ai.IsKnownRuntimeConfigKey(item.ConfigKey) {
			continue
		}

		filtered = append(filtered, item)
		configMap[item.ConfigKey] = item.ConfigValue
	}

	normalized := ai.NormalizeRuntimeConfig(configMap)
	return &AIConfigResponse{
		Configs:  normalized,
		Items:    filtered,
		Support:  buildAIConfigSupport(),
		Warnings: aiRuntime.RuntimeConfigIssues(normalized),
		Presets:  []AIPresetSummary{},
	}
}

// buildAIConfigItems 将运行时配置映射为后台配置项。
func buildAIConfigItems(configs map[string]string) []model.AdminConfig {
	items := make([]model.AdminConfig, 0, len(configs))
	for key, value := range ai.NormalizeRuntimeConfig(configs) {
		if !ai.IsKnownRuntimeConfigKey(key) {
			continue
		}

		items = append(items, model.AdminConfig{
			ConfigKey:   key,
			ConfigValue: value,
			ConfigType:  inferAIConfigType(key),
			Description: describeAIConfig(key),
		})
	}

	return items
}

// buildAIConfigSupport 返回后台 AI 配置页所需的当前支持范围说明。
func buildAIConfigSupport() AIConfigSupport {
	summary := aiRuntime.SupportedProviderSummary()
	return AIConfigSupport{
		PrimaryProviders:  summary.PrimaryProviders,
		FallbackProviders: summary.FallbackProviders,
		Notes:             summary.Notes,
	}
}

// buildAIPresetSummaries 将数据库预设记录整理为前端可直接消费的结构。
func buildAIPresetSummaries(presets []model.AIPreset) ([]AIPresetSummary, *uint, error) {
	summaries := make([]AIPresetSummary, 0, len(presets))
	var activePresetID *uint

	for _, preset := range presets {
		summary, err := buildAIPresetSummary(preset)
		if err != nil {
			return nil, nil, err
		}
		if summary.IsActive {
			presetID := summary.ID
			activePresetID = &presetID
		}
		summaries = append(summaries, summary)
	}

	return summaries, activePresetID, nil
}

// buildAIPresetSummary 将单条预设记录转换为响应摘要。
func buildAIPresetSummary(preset model.AIPreset) (AIPresetSummary, error) {
	configs, err := parseAIPresetConfigs(preset.ConfigJSON)
	if err != nil {
		return AIPresetSummary{}, err
	}

	return AIPresetSummary{
		ID:        preset.ID,
		Name:      preset.Name,
		IsActive:  preset.IsActive,
		UpdatedAt: preset.UpdatedAt,
		Configs:   configs,
	}, nil
}

// normalizeAIPresetName 统一清洗 AI 预设名称，避免首尾空白导致重复名称。
func normalizeAIPresetName(name string) string {
	return strings.TrimSpace(name)
}

// normalizeStoredAIConfigs 统一清洗并校验一份完整 AI 配置快照。
func normalizeStoredAIConfigs(configs map[string]string) (map[string]string, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("configs cannot be empty")
	}

	normalized := ai.NormalizeRuntimeConfig(configs)
	if err := aiRuntime.ValidateRuntimeConfig(normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

// parseAIPresetConfigs 从预设 JSON 中恢复完整 AI 配置快照。
func parseAIPresetConfigs(configJSON string) (map[string]string, error) {
	if strings.TrimSpace(configJSON) == "" {
		return nil, fmt.Errorf("preset config snapshot is empty")
	}

	var configs map[string]string
	if err := json.Unmarshal([]byte(configJSON), &configs); err != nil {
		return nil, fmt.Errorf("parse ai preset configs: %w", err)
	}
	return normalizeStoredAIConfigs(configs)
}

// serializeAIPresetConfigs 将完整 AI 配置快照序列化为稳定 JSON。
func serializeAIPresetConfigs(configs map[string]string) (string, error) {
	normalized, err := normalizeStoredAIConfigs(configs)
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal ai preset configs: %w", err)
	}
	return string(payload), nil
}

// inferAIConfigType 根据配置键推导后台表单应使用的字段类型。
func inferAIConfigType(key string) string {
	switch key {
	case ai.ConfigKeyTemperature, ai.ConfigKeyTopP, ai.ConfigKeyMaxTokens, ai.ConfigKeyTimeoutSeconds:
		return model.ConfigTypeNumber
	case ai.ConfigKeyEnableStream:
		return model.ConfigTypeBoolean
	default:
		return model.ConfigTypeString
	}
}

// describeAIConfig 为后台配置项生成简短说明文本。
func describeAIConfig(key string) string {
	switch key {
	case ai.ConfigKeyProvider:
		return "AI provider type"
	case ai.ConfigKeyModel:
		return "Default AI model"
	case ai.ConfigKeyAPIKey:
		return "AI upstream API key"
	case ai.ConfigKeyBaseURL:
		return "AI upstream base URL"
	case ai.ConfigKeyTemperature:
		return "Sampling temperature"
	case ai.ConfigKeyTopP:
		return "Top-p sampling"
	case ai.ConfigKeyMaxTokens:
		return "Maximum completion tokens"
	case ai.ConfigKeyTimeoutSeconds:
		return "AI request timeout in seconds"
	case ai.ConfigKeyEnableStream:
		return "Enable AI streaming"
	case ai.ConfigKeyFallbackProvider:
		return "Fallback provider when primary provider fails"
	case ai.ConfigKeyInterviewModel:
		return "Model override for interview scene"
	case ai.ConfigKeyPlanModel:
		return "Model override for learning plan scene"
	case ai.ConfigKeyCompanionModel:
		return "Model override for companion scene"
	case ai.ConfigKeyQuizModel:
		return "Model override for quiz analysis scene"
	default:
		return "AI runtime config"
	}
}
