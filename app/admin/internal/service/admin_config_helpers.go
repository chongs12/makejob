package service

import (
	"fmt"
	"strconv"
	"strings"

	kratoserr "github.com/go-kratos/kratos/v2/errors"

	adminv1 "makejob/api/makejob/admin/v1"
)

var aiDefaultConfigValues = map[string]string{
	"ai_provider":              "eino",
	"ai_fallback_provider":     "",
	"ai_model":                 "gpt-4o-mini",
	"ai_api_key":               "",
	"ai_base_url":              "",
	"ai_temperature":           "0.7",
	"ai_top_p":                 "0.9",
	"ai_max_tokens":            "2048",
	"ai_timeout_seconds":       "30",
	"ai_enable_stream":         "false",
	"ai_scene_interview_model": "",
	"ai_scene_plan_model":      "",
	"ai_scene_companion_model": "",
	"ai_scene_quiz_model":      "",
}

var aiSupportedPrimaryProviders = map[string]struct{}{
	"eino": {},
}

// defaultAIConfigValues 返回 AI 默认配置副本，避免调用方误修改共享常量。
func defaultAIConfigValues() map[string]string {
	result := make(map[string]string, len(aiDefaultConfigValues))
	for key, value := range aiDefaultConfigValues {
		result[key] = value
	}
	return result
}

// mergeAdminConfigItems 将数据库中的后台配置覆盖到基础默认值上，保持配置读取语义与单体一致。
func mergeAdminConfigItems(items []*adminv1.AdminConfigItem, base map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(items))
	for key, value := range base {
		merged[key] = value
	}
	for _, item := range items {
		if item == nil || item.Key == "" {
			continue
		}
		merged[item.Key] = strings.TrimSpace(item.Value)
	}
	return merged
}

// normalizeAIConfigInput 归一化并校验 AI 配置，避免保存非法值破坏运行时行为。
func normalizeAIConfigInput(input map[string]string) (map[string]string, error) {
	normalized := defaultAIConfigValues()
	for key, value := range input {
		if _, ok := aiDefaultConfigValues[key]; !ok {
			return nil, kratoserr.BadRequest("INVALID_AI_CONFIG", fmt.Sprintf("不支持的 AI 配置键: %s", key))
		}
		normalized[key] = strings.TrimSpace(value)
	}

	normalized["ai_provider"] = strings.ToLower(normalized["ai_provider"])
	normalized["ai_fallback_provider"] = strings.ToLower(normalized["ai_fallback_provider"])
	normalized["ai_enable_stream"] = strings.ToLower(normalized["ai_enable_stream"])

	if _, ok := aiSupportedPrimaryProviders[normalized["ai_provider"]]; !ok {
		return nil, kratoserr.BadRequest("INVALID_AI_PROVIDER", "当前仅支持 ai_provider=eino")
	}
	if normalized["ai_fallback_provider"] != "" {
		return nil, kratoserr.BadRequest("INVALID_AI_FALLBACK_PROVIDER", "当前不支持 ai_fallback_provider")
	}
	if strings.TrimSpace(normalized["ai_model"]) == "" {
		return nil, kratoserr.BadRequest("INVALID_AI_MODEL", "ai_model 不能为空")
	}
	if err := validateFloatRange("ai_temperature", normalized["ai_temperature"], 0, 2, false); err != nil {
		return nil, err
	}
	if err := validateFloatRange("ai_top_p", normalized["ai_top_p"], 0, 1, true); err != nil {
		return nil, err
	}
	if err := validatePositiveInt("ai_max_tokens", normalized["ai_max_tokens"]); err != nil {
		return nil, err
	}
	if err := validatePositiveInt("ai_timeout_seconds", normalized["ai_timeout_seconds"]); err != nil {
		return nil, err
	}
	if normalized["ai_enable_stream"] != "true" && normalized["ai_enable_stream"] != "false" {
		return nil, kratoserr.BadRequest("INVALID_AI_ENABLE_STREAM", "ai_enable_stream 仅支持 true 或 false")
	}

	return normalized, nil
}

// validateFloatRange 校验浮点型配置值是否位于合法区间内。
func validateFloatRange(key, raw string, min, max float64, minExclusive bool) error {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return kratoserr.BadRequest("INVALID_AI_CONFIG", fmt.Sprintf("%s 必须是数字", key))
	}
	if minExclusive {
		if value <= min || value > max {
			return kratoserr.BadRequest("INVALID_AI_CONFIG", fmt.Sprintf("%s 必须大于 %v 且不超过 %v", key, min, max))
		}
		return nil
	}
	if value < min || value > max {
		return kratoserr.BadRequest("INVALID_AI_CONFIG", fmt.Sprintf("%s 必须在 %v 到 %v 之间", key, min, max))
	}
	return nil
}

// validatePositiveInt 校验整型配置值是否为正整数。
func validatePositiveInt(key, raw string) error {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return kratoserr.BadRequest("INVALID_AI_CONFIG", fmt.Sprintf("%s 必须是正整数", key))
	}
	return nil
}
