package service

import (
	"context"
	"testing"

	"makejob-backend/internal/model"
)

// TestLive2DServiceGetCurrentModelPreferIndustryModel 验证行业专属模型优先于通用模型。
func TestLive2DServiceGetCurrentModelPreferIndustryModel(t *testing.T) {
	svc := NewLive2DService(
		&mockLive2DModelRepository{
			models: []model.Live2DModel{
				{
					Name:     "通用面试模型",
					Scene:    model.Live2DSceneInterview,
					ModelURL: "/live2d-assets/generic/model.json",
					IsActive: true,
				},
				{
					Name:         "Go 面试模型",
					Scene:        model.Live2DSceneInterview,
					IndustryID:   2,
					ModelURL:     "/live2d-assets/go/model.json",
					ThumbnailURL: "/live2d-assets/go/cover.png",
					ConfigJSON:   `{"scale":0.52,"tap_motion":"focus"}`,
					IsActive:     true,
				},
			},
		},
		&mockIndustryRepository{
			byCode: map[string]*model.Industry{
				"go": {BaseModel: model.BaseModel{ID: 2}, Code: "go", Name: "Go"},
			},
		},
	)

	resp, err := svc.GetCurrentModel(context.Background(), &CurrentLive2DModelRequest{
		Scene:        model.Live2DSceneInterview,
		IndustryCode: "go",
	})
	if err != nil {
		t.Fatalf("GetCurrentModel returned error: %v", err)
	}

	if resp.Source != "database" {
		t.Fatalf("expected database source, got %s", resp.Source)
	}
	if resp.Name != "Go 面试模型" {
		t.Fatalf("expected specific model, got %s", resp.Name)
	}
	if resp.IndustryCode != "go" {
		t.Fatalf("expected industry code go, got %s", resp.IndustryCode)
	}
	if resp.ModelURL != "/live2d-assets/go/model.json" {
		t.Fatalf("unexpected model url: %s", resp.ModelURL)
	}
	if scale, ok := resp.Config["scale"].(float64); !ok || scale != 0.52 {
		t.Fatalf("expected overridden scale 0.52, got %#v", resp.Config["scale"])
	}
	if voiceSource, ok := resp.Config["voice_source"].(string); !ok || voiceSource != "volcengine" {
		t.Fatalf("expected default voice source, got %#v", resp.Config["voice_source"])
	}
}

// TestLive2DServiceGetCurrentModelFallbackToGeneric 验证未命中行业模型时会回退到通用模型。
func TestLive2DServiceGetCurrentModelFallbackToGeneric(t *testing.T) {
	svc := NewLive2DService(
		&mockLive2DModelRepository{
			models: []model.Live2DModel{
				{
					Name:         "通用陪伴模型",
					Scene:        model.Live2DSceneCompanion,
					ModelURL:     "/live2d-assets/companion/model.json",
					ThumbnailURL: "/live2d-assets/companion/cover.png",
					IsActive:     true,
				},
				{
					Name:       "Java 陪伴模型",
					Scene:      model.Live2DSceneCompanion,
					IndustryID: 9,
					ModelURL:   "/live2d-assets/java/model.json",
					IsActive:   false,
				},
			},
		},
		&mockIndustryRepository{
			byCode: map[string]*model.Industry{
				"java": {BaseModel: model.BaseModel{ID: 9}, Code: "java", Name: "Java"},
			},
		},
	)

	resp, err := svc.GetCurrentModel(context.Background(), &CurrentLive2DModelRequest{
		Scene:        model.Live2DSceneCompanion,
		IndustryCode: "java",
	})
	if err != nil {
		t.Fatalf("GetCurrentModel returned error: %v", err)
	}

	if resp.Name != "通用陪伴模型" {
		t.Fatalf("expected generic model, got %s", resp.Name)
	}
	if resp.IndustryCode != "" {
		t.Fatalf("expected empty industry code for generic model, got %s", resp.IndustryCode)
	}
	if resp.Source != "database" {
		t.Fatalf("expected database source, got %s", resp.Source)
	}
}

// TestLive2DServiceGetCurrentModelFallbackToBundled 验证仓库为空时会回退到内置模型。
func TestLive2DServiceGetCurrentModelFallbackToBundled(t *testing.T) {
	svc := NewLive2DService(nil, nil)

	resp, err := svc.GetCurrentModel(context.Background(), &CurrentLive2DModelRequest{
		Scene: model.Live2DSceneCompanion,
	})
	if err != nil {
		t.Fatalf("GetCurrentModel returned error: %v", err)
	}

	if resp.Source != "bundled" {
		t.Fatalf("expected bundled source, got %s", resp.Source)
	}
	if resp.Name != bundledLive2DName {
		t.Fatalf("expected bundled model %s, got %s", bundledLive2DName, resp.Name)
	}
	if resp.ModelURL == "" {
		t.Fatalf("expected bundled model url")
	}
	if resp.Config["voice_source"] != "volcengine" {
		t.Fatalf("expected volcengine voice source, got %#v", resp.Config["voice_source"])
	}
}

// mockLive2DModelRepository 模拟 Live2D 模型仓库。
type mockLive2DModelRepository struct {
	models []model.Live2DModel
}

// List 返回预置的 Live2D 模型列表。
func (m *mockLive2DModelRepository) List(ctx context.Context) ([]model.Live2DModel, error) {
	return m.models, nil
}

// GetByID 返回空结果以满足接口。
func (m *mockLive2DModelRepository) GetByID(ctx context.Context, id uint) (*model.Live2DModel, error) {
	return nil, nil
}

// Create 不在当前测试中使用。
func (m *mockLive2DModelRepository) Create(ctx context.Context, live2d *model.Live2DModel) error {
	return nil
}

// Update 不在当前测试中使用。
func (m *mockLive2DModelRepository) Update(ctx context.Context, live2d *model.Live2DModel) error {
	return nil
}

// Delete 不在当前测试中使用。
func (m *mockLive2DModelRepository) Delete(ctx context.Context, id uint) error {
	return nil
}

// mockIndustryRepository 模拟行业仓库。
type mockIndustryRepository struct {
	byCode map[string]*model.Industry
}

// List 返回空列表以满足接口。
func (m *mockIndustryRepository) List(ctx context.Context) ([]model.Industry, error) {
	return nil, nil
}

// GetByID 返回空结果以满足接口。
func (m *mockIndustryRepository) GetByID(ctx context.Context, id uint) (*model.Industry, error) {
	return nil, nil
}

// Create 不在当前测试中使用。
func (m *mockIndustryRepository) Create(ctx context.Context, industry *model.Industry) error {
	return nil
}

// Update 不在当前测试中使用。
func (m *mockIndustryRepository) Update(ctx context.Context, industry *model.Industry) error {
	return nil
}

// GetByCode 返回预置的行业信息。
func (m *mockIndustryRepository) GetByCode(ctx context.Context, code string) (*model.Industry, error) {
	if m == nil || m.byCode == nil {
		return nil, nil
	}
	return m.byCode[code], nil
}
