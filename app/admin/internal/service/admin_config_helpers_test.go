package service

import (
	"strings"
	"testing"
)

import adminv1 "makejob/api/makejob/admin/v1"

// TestMergeAdminConfigItems 验证数据库配置会覆盖基础默认值，同时保留未显式配置的默认项。
func TestMergeAdminConfigItems(t *testing.T) {
	result := mergeAdminConfigItems([]*adminv1.AdminConfigItem{
		{Key: "ai_model", Value: "custom-model"},
		{Key: "ai_timeout_seconds", Value: "60"},
	}, map[string]string{
		"ai_model":           "default-model",
		"ai_timeout_seconds": "30",
		"ai_provider":        "eino",
	})

	if result["ai_model"] != "custom-model" {
		t.Fatalf("expected ai_model override, got %q", result["ai_model"])
	}
	if result["ai_timeout_seconds"] != "60" {
		t.Fatalf("expected ai_timeout_seconds override, got %q", result["ai_timeout_seconds"])
	}
	if result["ai_provider"] != "eino" {
		t.Fatalf("expected default ai_provider to be preserved, got %q", result["ai_provider"])
	}
}

// TestNormalizeAdminAIConfigsRejectsUnsupportedProvider 验证 admin 服务会通过 bridge 复用单体规则拒绝非法 AI 配置。
func TestNormalizeAdminAIConfigsRejectsUnsupportedProvider(t *testing.T) {
	_, err := normalizeAdminAIConfigs(map[string]string{
		"ai_provider":        "openai",
		"ai_model":           "gpt-4o-mini",
		"ai_api_key":         "key",
		"ai_base_url":        "https://example.com",
		"ai_timeout_seconds": "30",
	})
	if err == nil {
		t.Fatal("expected invalid provider config to be rejected")
	}
	if !strings.Contains(err.Error(), "Provider") {
		t.Fatalf("expected provider validation error, got %v", err)
	}
}
