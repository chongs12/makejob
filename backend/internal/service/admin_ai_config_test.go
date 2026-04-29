package service

import (
	"testing"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

// TestBuildAIConfigResponseMergesBaseConfig 验证响应会合并配置文件默认值与后台覆盖值。
func TestBuildAIConfigResponseMergesBaseConfig(t *testing.T) {
	response := buildAIConfigResponse([]model.AdminConfig{
		{ConfigKey: "ai_provider", ConfigValue: "mock"},
		{ConfigKey: "ai_scene_interview_model", ConfigValue: "db-interview-model"},
	}, map[string]string{
		"ai_provider":              "eino",
		"ai_model":                 "cfg-default-model",
		"ai_scene_interview_model": "cfg-interview-model",
	})

	if got := response.Configs["ai_provider"]; got != "mock" {
		t.Fatalf("expected db ai_provider override to be mock, got %q", got)
	}
	if got := response.Configs["ai_model"]; got != "cfg-default-model" {
		t.Fatalf("expected base ai_model to be preserved, got %q", got)
	}
	if got := response.Configs["ai_scene_interview_model"]; got != "db-interview-model" {
		t.Fatalf("expected db interview model override, got %q", got)
	}
	if len(response.Support.PrimaryProviders) != 1 || response.Support.PrimaryProviders[0] != string(ai.ProviderTypeEino) {
		t.Fatalf("expected eino to be the only supported primary provider, got %#v", response.Support.PrimaryProviders)
	}
	if len(response.Warnings) == 0 {
		t.Fatalf("expected legacy mock provider to produce warnings")
	}
}
