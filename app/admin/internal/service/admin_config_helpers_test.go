package service

import (
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
