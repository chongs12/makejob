package service

import (
	"makejob-backend/internal/ai"
	aiRuntime "makejob-backend/internal/ai/runtime"
	"makejob-backend/internal/model"
)

type AIConfigSupport struct {
	PrimaryProviders  []string `json:"primary_providers"`
	FallbackProviders []string `json:"fallback_providers"`
	Notes             []string `json:"notes"`
}

type AIConfigResponse struct {
	Configs  map[string]string   `json:"configs"`
	Items    []model.AdminConfig `json:"items"`
	Support  AIConfigSupport     `json:"support"`
	Warnings []string            `json:"warnings"`
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
