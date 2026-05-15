package runtime

import (
	"context"
	"testing"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

type fakePromptTemplateRepo struct {
	items []model.PromptTemplate
}

func (r *fakePromptTemplateRepo) List(_ context.Context, industryID *uint, scene string) ([]model.PromptTemplate, error) {
	/* 返回符合条件的模板列表 */
	result := make([]model.PromptTemplate, 0, len(r.items))
	for _, item := range r.items {
		if scene != "" && item.Scene != scene {
			continue
		}
		switch {
		case industryID == nil && item.IndustryID != nil:
			continue
		case industryID != nil && (item.IndustryID == nil || *item.IndustryID != *industryID):
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (r *fakePromptTemplateRepo) GetByID(_ context.Context, id uint) (*model.PromptTemplate, error) {
	/* 返回指定模板 */
	for i := range r.items {
		if r.items[i].ID == id {
			return &r.items[i], nil
		}
	}
	return nil, nil
}

func (r *fakePromptTemplateRepo) Create(_ context.Context, _ *model.PromptTemplate) error {
	/* 创建模板测试桩 */
	return nil
}

func (r *fakePromptTemplateRepo) Update(_ context.Context, _ *model.PromptTemplate) error {
	/* 更新模板测试桩 */
	return nil
}

func (r *fakePromptTemplateRepo) Delete(_ context.Context, _ uint) error {
	/* 删除模板测试桩 */
	return nil
}

type fakeIndustryRepo struct{}

func (r *fakeIndustryRepo) List(_ context.Context) ([]model.Industry, error) {
	/* 返回行业列表测试桩 */
	return nil, nil
}

func (r *fakeIndustryRepo) GetByID(_ context.Context, id uint) (*model.Industry, error) {
	/* 按 ID 返回行业测试桩 */
	return &model.Industry{BaseModel: model.BaseModel{ID: id}}, nil
}

func (r *fakeIndustryRepo) Create(_ context.Context, _ *model.Industry) error {
	/* 创建行业测试桩 */
	return nil
}

func (r *fakeIndustryRepo) Update(_ context.Context, _ *model.Industry) error {
	/* 更新行业测试桩 */
	return nil
}

func (r *fakeIndustryRepo) GetByCode(_ context.Context, _ string) (*model.Industry, error) {
	/* 按代码返回行业测试桩 */
	return nil, nil
}

func TestDebuggerRunUsesCustomTemplate(t *testing.T) {
	/* 验证自定义模板调试会直接渲染且脱敏配置 */
	debugger := NewDebugger(
		&fakeAdminConfigRepo{
			items: []model.AdminConfig{
				{ConfigKey: "ai_api_key", ConfigValue: "secret-token-value"},
			},
		},
		nil,
		nil,
		map[string]string{
			"ai_provider": "eino",
			"ai_model":    "test-model",
		},
	)

	result, err := debugger.Run(context.Background(), DebugRequest{
		Scene:           model.PromptSceneQuiz,
		TemplateContent: "请分析 {{topic}}",
		Variables: map[string]string{
			"topic": "并发安全",
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.PromptSource != "template_custom" {
		t.Fatalf("expected prompt source template_custom, got %q", result.PromptSource)
	}
	if result.RenderedPrompt != "请分析 并发安全" {
		t.Fatalf("unexpected rendered prompt: %q", result.RenderedPrompt)
	}
	if result.RuntimeConfig["ai_api_key"] == "secret-token-value" {
		t.Fatalf("expected api key to be masked")
	}
}

func TestDebuggerRunPrefersIndustryPrompt(t *testing.T) {
	/* 验证行业模板优先于通用模板 */
	industryID := uint(2)
	debugger := NewDebugger(
		nil,
		&fakePromptTemplateRepo{
			items: []model.PromptTemplate{
				{
					BaseModel:       model.BaseModel{ID: 1},
					Name:            "通用模板",
					Scene:           model.PromptScenePlan,
					TemplateContent: "通用 {{goal}}",
					IsActive:        true,
				},
				{
					BaseModel:       model.BaseModel{ID: 2},
					IndustryID:      &industryID,
					Name:            "行业模板",
					Scene:           model.PromptScenePlan,
					TemplateContent: "行业 {{goal}}",
					IsActive:        true,
				},
			},
		},
		&fakeIndustryRepo{},
		map[string]string{
			"ai_provider": "eino",
		},
	)

	result, err := debugger.Run(context.Background(), DebugRequest{
		Scene:      model.PromptScenePlan,
		IndustryID: &industryID,
		Variables: map[string]string{
			"goal": "准备面试",
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.PromptSource != "template_industry" {
		t.Fatalf("expected industry prompt source, got %q", result.PromptSource)
	}
	if result.SelectedPromptID == nil || *result.SelectedPromptID != 2 {
		t.Fatalf("expected selected prompt id 2, got %#v", result.SelectedPromptID)
	}
	if result.RenderedPrompt != "行业 准备面试" {
		t.Fatalf("unexpected rendered prompt: %q", result.RenderedPrompt)
	}
}

func TestDebuggerRunRejectsRemovedMockProvider(t *testing.T) {
	/* 验证调试器在遇到已移除的 mock provider 时会返回明确错误 */
	debugger := NewDebugger(
		nil,
		nil,
		nil,
		map[string]string{
			"ai_provider": "mock",
			"ai_model":    "mock-model",
		},
	)

	result, err := debugger.Run(context.Background(), DebugRequest{
		Scene:    model.PromptSceneCompanion,
		RunModel: true,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.RequestMessages) == 0 {
		t.Fatalf("expected request messages to be built")
	}
	if result.ModelOutput != "" {
		t.Fatalf("expected model output to be empty, got %q", result.ModelOutput)
	}
	if result.ModelError == "" {
		t.Fatalf("expected model error to be returned")
	}
	if result.Provider != "unavailable" {
		t.Fatalf("expected provider unavailable, got %q", result.Provider)
	}
}

func TestApplyDebugRuntimeOverridesPreservesExistingSecrets(t *testing.T) {
	/* 验证仅覆盖显式传入字段，不会把现有 api_key/base_url/model 覆盖成默认值 */
	runtimeConfig := map[string]string{
		ai.ConfigKeyProvider:       "eino",
		ai.ConfigKeyModel:          "MiniMax-M2.7",
		ai.ConfigKeyAPIKey:         "secret-token",
		ai.ConfigKeyBaseURL:        "https://api.minimaxi.com/v1",
		ai.ConfigKeyTimeoutSeconds: "30",
	}

	merged := applyDebugRuntimeOverrides(runtimeConfig, map[string]string{
		ai.ConfigKeyTimeoutSeconds: "90",
		"max_tokens":               "1400",
	})

	if merged[ai.ConfigKeyAPIKey] != "secret-token" {
		t.Fatalf("expected api key to be preserved, got %q", merged[ai.ConfigKeyAPIKey])
	}
	if merged[ai.ConfigKeyBaseURL] != "https://api.minimaxi.com/v1" {
		t.Fatalf("expected base url to be preserved, got %q", merged[ai.ConfigKeyBaseURL])
	}
	if merged[ai.ConfigKeyModel] != "MiniMax-M2.7" {
		t.Fatalf("expected model to be preserved, got %q", merged[ai.ConfigKeyModel])
	}
	if merged[ai.ConfigKeyTimeoutSeconds] != "90" {
		t.Fatalf("expected timeout override to apply, got %q", merged[ai.ConfigKeyTimeoutSeconds])
	}
	if merged[ai.ConfigKeyMaxTokens] != "1400" {
		t.Fatalf("expected max_tokens override to apply, got %q", merged[ai.ConfigKeyMaxTokens])
	}
}
