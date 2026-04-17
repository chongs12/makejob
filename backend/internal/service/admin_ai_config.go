package service

import (
	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

type AIConfigResponse struct {
	Configs map[string]string   `json:"configs"`
	Items   []model.AdminConfig `json:"items"`
}

func buildAIConfigResponse(items []model.AdminConfig) *AIConfigResponse {
	filtered := make([]model.AdminConfig, 0, len(items))
	configMap := make(map[string]string, len(items))

	for _, item := range items {
		if !ai.IsRuntimeConfigKey(item.ConfigKey) {
			continue
		}

		filtered = append(filtered, item)
		configMap[item.ConfigKey] = item.ConfigValue
	}

	return &AIConfigResponse{
		Configs: ai.NormalizeRuntimeConfig(configMap),
		Items:   filtered,
	}
}

func buildAIConfigItems(configs map[string]string) []model.AdminConfig {
	items := make([]model.AdminConfig, 0, len(configs))
	for key, value := range ai.NormalizeRuntimeConfig(configs) {
		if !ai.IsRuntimeConfigKey(key) {
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
