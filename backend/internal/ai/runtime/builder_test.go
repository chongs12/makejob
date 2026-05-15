package runtime

import (
	"context"
	"testing"

	"makejob-backend/internal/model"
)

type fakeAdminConfigRepo struct {
	items []model.AdminConfig
}

// List 返回预置配置列表。
func (r *fakeAdminConfigRepo) List(context.Context) ([]model.AdminConfig, error) {
	return r.items, nil
}

// GetByKey 返回空结果，当前测试不依赖该方法。
func (r *fakeAdminConfigRepo) GetByKey(context.Context, string) (*model.AdminConfig, error) {
	return nil, nil
}

// Upsert 为测试桩实现。
func (r *fakeAdminConfigRepo) Upsert(context.Context, *model.AdminConfig) error {
	return nil
}

// BatchUpsert 为测试桩实现。
func (r *fakeAdminConfigRepo) BatchUpsert(context.Context, []model.AdminConfig) error {
	return nil
}

// TestLoadRuntimeConfigPrefersAdminConfig 验证后台配置优先于 config.yaml 默认值。
func TestLoadRuntimeConfigPrefersAdminConfig(t *testing.T) {
	builder := &Builder{
		configRepo: &fakeAdminConfigRepo{
			items: []model.AdminConfig{
				{ConfigKey: "ai_provider", ConfigValue: "azure"},
				{ConfigKey: "ai_model", ConfigValue: "db-model"},
				{ConfigKey: "ai_timeout_seconds", ConfigValue: "45"},
			},
		},
		baseConfig: map[string]string{
			"ai_provider":          "eino",
			"ai_model":             "cfg-model",
			"ai_timeout_seconds":   "30",
			"ai_fallback_provider": "",
		},
	}

	config := builder.loadRuntimeConfig(context.Background())

	if got := config["ai_provider"]; got != "azure" {
		t.Fatalf("expected ai_provider=azure, got %q", got)
	}
	if got := config["ai_model"]; got != "db-model" {
		t.Fatalf("expected ai_model=db-model, got %q", got)
	}
	if got := config["ai_timeout_seconds"]; got != "45" {
		t.Fatalf("expected ai_timeout_seconds=45, got %q", got)
	}
}

// TestBuildSceneConfigAppliesSceneModelOverride 验证场景模型覆盖逻辑生效。
func TestBuildSceneConfigAppliesSceneModelOverride(t *testing.T) {
	config := buildSceneConfig(map[string]string{
		"ai_model":                 "default-model",
		"ai_scene_interview_model": "interview-model",
	}, model.PromptSceneInterview)

	if got := config["ai_model"]; got != "interview-model" {
		t.Fatalf("expected ai_model=interview-model, got %q", got)
	}
}
