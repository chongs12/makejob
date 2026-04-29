package ai

import "strings"

const (
	ConfigKeyProvider         = "ai_provider"
	ConfigKeyModel            = "ai_model"
	ConfigKeyAPIKey           = "ai_api_key"
	ConfigKeyBaseURL          = "ai_base_url"
	ConfigKeyTemperature      = "ai_temperature"
	ConfigKeyTopP             = "ai_top_p"
	ConfigKeyMaxTokens        = "ai_max_tokens"
	ConfigKeyTimeoutSeconds   = "ai_timeout_seconds"
	ConfigKeyEnableStream     = "ai_enable_stream"
	ConfigKeyFallbackProvider = "ai_fallback_provider"
	ConfigKeyInterviewModel   = "ai_scene_interview_model"
	ConfigKeyPlanModel        = "ai_scene_plan_model"
	ConfigKeyCompanionModel   = "ai_scene_companion_model"
	ConfigKeyQuizModel        = "ai_scene_quiz_model"
)

var defaultRuntimeConfig = map[string]string{
	ConfigKeyProvider:         string(ProviderTypeEino),
	ConfigKeyModel:            "gpt-4o-mini",
	ConfigKeyAPIKey:           "",
	ConfigKeyBaseURL:          "",
	ConfigKeyTemperature:      "0.7",
	ConfigKeyTopP:             "0.9",
	ConfigKeyMaxTokens:        "2048",
	ConfigKeyTimeoutSeconds:   "30",
	ConfigKeyEnableStream:     "false",
	ConfigKeyFallbackProvider: "",
	ConfigKeyInterviewModel:   "",
	ConfigKeyPlanModel:        "",
	ConfigKeyCompanionModel:   "",
	ConfigKeyQuizModel:        "",
}

var legacyRuntimeAliases = map[string]string{
	"provider":        ConfigKeyProvider,
	"model_name":      ConfigKeyModel,
	"api_key":         ConfigKeyAPIKey,
	"base_url":        ConfigKeyBaseURL,
	"temperature":     ConfigKeyTemperature,
	"top_p":           ConfigKeyTopP,
	"max_tokens":      ConfigKeyMaxTokens,
	"interview_model": ConfigKeyInterviewModel,
	"plan_model":      ConfigKeyPlanModel,
	"companion_model": ConfigKeyCompanionModel,
	"quiz_model":      ConfigKeyQuizModel,
}

// DefaultRuntimeConfig 返回 AI runtime 的默认配置副本，避免调用方误改全局默认值。
func DefaultRuntimeConfig() map[string]string {
	result := make(map[string]string, len(defaultRuntimeConfig))
	for key, value := range defaultRuntimeConfig {
		result[key] = value
	}
	return result
}

// IsRuntimeConfigKey 判断配置键是否属于 AI runtime 命名空间。
func IsRuntimeConfigKey(key string) bool {
	return strings.HasPrefix(key, "ai_")
}

// IsKnownRuntimeConfigKey 判断配置键是否属于当前已知的 runtime 配置集合。
func IsKnownRuntimeConfigKey(key string) bool {
	if _, ok := defaultRuntimeConfig[key]; ok {
		return true
	}
	_, ok := legacyRuntimeAliases[key]
	return ok
}

// NormalizeRuntimeConfig 统一整理 runtime 配置键和值，并补齐默认值。
func NormalizeRuntimeConfig(raw map[string]string) map[string]string {
	normalized := DefaultRuntimeConfig()

	for key, value := range raw {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		if alias, ok := legacyRuntimeAliases[key]; ok {
			normalized[alias] = trimmed
			continue
		}

		if IsRuntimeConfigKey(key) {
			normalized[key] = trimmed
		}
	}

	normalized[ConfigKeyProvider] = NormalizeProviderType(normalized[ConfigKeyProvider])
	normalized[ConfigKeyFallbackProvider] = NormalizeProviderType(normalized[ConfigKeyFallbackProvider])

	return normalized
}

// KnownRuntimeConfigKeys 返回当前已知的 runtime 配置键列表副本。
func KnownRuntimeConfigKeys() []string {
	keys := make([]string, 0, len(defaultRuntimeConfig))
	for key := range defaultRuntimeConfig {
		keys = append(keys, key)
	}
	return keys
}

// NormalizeProviderType 规范化 Provider 类型名称，无法识别时返回空字符串。
func NormalizeProviderType(value string) string {
	switch ProviderType(strings.ToLower(strings.TrimSpace(value))) {
	case ProviderTypeEino:
		return string(ProviderTypeEino)
	case ProviderTypeMock:
		return string(ProviderTypeMock)
	case ProviderTypeOpenAI:
		return string(ProviderTypeOpenAI)
	case ProviderTypeAzure:
		return string(ProviderTypeAzure)
	default:
		return ""
	}
}
