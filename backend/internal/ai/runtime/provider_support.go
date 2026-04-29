package runtime

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"makejob-backend/internal/ai"
)

// ProviderSupportSummary 描述当前 runtime 对外暴露的 Provider 支持范围。
type ProviderSupportSummary struct {
	PrimaryProviders  []string `json:"primary_providers"`
	FallbackProviders []string `json:"fallback_providers"`
	Notes             []string `json:"notes"`
}

var supportedPrimaryProviders = []string{
	string(ai.ProviderTypeEino),
}

var supportedFallbackProviders = []string{}

// SupportedProviderSummary 返回当前 runtime 真实支持的 Provider 范围与说明。
func SupportedProviderSummary() ProviderSupportSummary {
	return ProviderSupportSummary{
		PrimaryProviders:  cloneStringSlice(supportedPrimaryProviders),
		FallbackProviders: cloneStringSlice(supportedFallbackProviders),
		Notes: []string{
			"当前 runtime 仅支持 `eino` 作为主 Provider。",
			"当前未接入额外兜底 Provider，`ai_fallback_provider` 仅支持留空。",
			"`openai` 与 `azure` 仍是预留配置项，尚未接入运行时创建逻辑。",
			"`mock` 已从运行时移除，不允许继续作为 AI Provider 使用。",
		},
	}
}

// RuntimeConfigIssues 收集当前 runtime 配置中的已知问题，便于后台在保存前和展示时提示。
func RuntimeConfigIssues(config map[string]string) []string {
	issues := make([]string, 0)

	for key := range config {
		if !ai.IsKnownRuntimeConfigKey(key) {
			issues = append(issues, fmt.Sprintf("不支持的 AI 配置键: %s", strings.TrimSpace(key)))
		}
	}

	normalized := ai.NormalizeRuntimeConfig(config)
	primaryProvider := strings.TrimSpace(normalized[ai.ConfigKeyProvider])
	if primaryProvider == "" {
		issues = append(issues, "必须配置 ai_provider")
	} else if !containsString(supportedPrimaryProviders, primaryProvider) {
		issues = append(issues, fmt.Sprintf("当前 runtime 不支持主 Provider `%s`", primaryProvider))
	}

	fallbackProvider := strings.TrimSpace(normalized[ai.ConfigKeyFallbackProvider])
	if fallbackProvider != "" {
		if !containsString(supportedFallbackProviders, fallbackProvider) {
			issues = append(issues, fmt.Sprintf("当前 runtime 不支持兜底 Provider `%s`", fallbackProvider))
		}
		if fallbackProvider == primaryProvider {
			issues = append(issues, "兜底 Provider 不能与主 Provider 相同")
		}
	}

	if primaryProvider == string(ai.ProviderTypeEino) {
		if strings.TrimSpace(normalized[ai.ConfigKeyModel]) == "" {
			issues = append(issues, fmt.Sprintf("Provider `%s` 需要配置 %s", primaryProvider, ai.ConfigKeyModel))
		}
		if strings.TrimSpace(normalized[ai.ConfigKeyAPIKey]) == "" {
			issues = append(issues, fmt.Sprintf("Provider `%s` 需要配置 %s", primaryProvider, ai.ConfigKeyAPIKey))
		}
	}

	if raw := strings.TrimSpace(normalized[ai.ConfigKeyTemperature]); raw != "" {
		if value, err := strconv.ParseFloat(raw, 64); err != nil || value < 0 || value > 2 {
			issues = append(issues, fmt.Sprintf("%s 需要是 0 到 2 之间的数字", ai.ConfigKeyTemperature))
		}
	}

	if raw := strings.TrimSpace(normalized[ai.ConfigKeyTopP]); raw != "" {
		if value, err := strconv.ParseFloat(raw, 64); err != nil || value <= 0 || value > 1 {
			issues = append(issues, fmt.Sprintf("%s 需要是大于 0 且不超过 1 的数字", ai.ConfigKeyTopP))
		}
	}

	if raw := strings.TrimSpace(normalized[ai.ConfigKeyMaxTokens]); raw != "" {
		if value, err := strconv.Atoi(raw); err != nil || value <= 0 {
			issues = append(issues, fmt.Sprintf("%s 需要是正整数", ai.ConfigKeyMaxTokens))
		}
	}

	if raw := strings.TrimSpace(normalized[ai.ConfigKeyTimeoutSeconds]); raw != "" {
		if value, err := strconv.Atoi(raw); err != nil || value <= 0 {
			issues = append(issues, fmt.Sprintf("%s 需要是正整数", ai.ConfigKeyTimeoutSeconds))
		}
	}

	if raw := strings.TrimSpace(normalized[ai.ConfigKeyEnableStream]); raw != "" {
		lowered := strings.ToLower(raw)
		if lowered != "true" && lowered != "false" {
			issues = append(issues, fmt.Sprintf("%s 仅支持 true 或 false", ai.ConfigKeyEnableStream))
		}
	}

	return dedupeStrings(issues)
}

// ValidateRuntimeConfig 校验 runtime 配置是否与当前真实支持范围一致。
func ValidateRuntimeConfig(config map[string]string) error {
	issues := RuntimeConfigIssues(config)
	if len(issues) == 0 {
		return nil
	}
	return errors.New(strings.Join(issues, "；"))
}

// cloneStringSlice 复制字符串切片，避免调用方修改共享常量。
func cloneStringSlice(values []string) []string {
	result := make([]string, len(values))
	copy(result, values)
	return result
}

// containsString 判断切片中是否包含指定字符串。
func containsString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

// dedupeStrings 对字符串列表去重并忽略空白值，避免页面提示重复。
func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
