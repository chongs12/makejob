package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"makejob-backend/internal/common"
	"makejob-backend/internal/live2dassets"
	"makejob-backend/internal/model"
)

// TestLive2DServiceGetCurrentModelPreferIndustryModel 验证行业专属模型优先于通用模型。
func TestLive2DServiceGetCurrentModelPreferIndustryModel(t *testing.T) {
	industryID := uint(2)
	ttsConfigID := uint(18)
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
					IndustryID:   &industryID,
					ModelURL:     "/live2d-assets/go/model.json",
					ThumbnailURL: "/live2d-assets/go/cover.png",
					ConfigJSON:   `{"scale":0.52,"tap_motion":"focus"}`,
					TTSConfigID:  &ttsConfigID,
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
	if resolvedTTSConfigID, ok := resp.Config["tts_config_id"].(uint); !ok || resolvedTTSConfigID != ttsConfigID {
		t.Fatalf("expected propagated tts_config_id %d, got %#v", ttsConfigID, resp.Config["tts_config_id"])
	}
}

// TestLive2DServiceGetCurrentModelFallbackToGeneric 验证未命中行业模型时会回退到通用模型。
func TestLive2DServiceGetCurrentModelFallbackToGeneric(t *testing.T) {
	industryID := uint(9)
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
					IndustryID: &industryID,
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

// TestLive2DServiceListSelectableModels 验证前台切换列表只返回后台已入库且启用的模型。
func TestLive2DServiceListSelectableModels(t *testing.T) {
	goIndustryID := uint(2)
	javaIndustryID := uint(9)

	svc := NewLive2DService(
		&mockLive2DModelRepository{
			models: []model.Live2DModel{
				{
					BaseModel:  model.BaseModel{ID: 101},
					Name:       "Go 专属陪伴模型",
					Scene:      model.Live2DSceneCompanion,
					IndustryID: &goIndustryID,
					ModelURL:   "/live2d-assets/go/companion.model3.json",
					ConfigJSON: `{"background_image_url":"/live2d-assets/backgrounds/go-room.webp"}`,
					IsActive:   true,
				},
				{
					BaseModel: model.BaseModel{ID: 102},
					Name:      "通用陪伴模型",
					Scene:     model.Live2DSceneCompanion,
					ModelURL:  "/live2d-assets/generic/companion.model3.json",
					IsActive:  true,
				},
				{
					BaseModel:  model.BaseModel{ID: 103},
					Name:       "Java 陪伴模型",
					Scene:      model.Live2DSceneCompanion,
					IndustryID: &javaIndustryID,
					ModelURL:   "/live2d-assets/java/companion.model3.json",
					IsActive:   true,
				},
			},
		},
		&mockIndustryRepository{
			byCode: map[string]*model.Industry{
				"go": {BaseModel: model.BaseModel{ID: 2}, Code: "go", Name: "Go"},
			},
		},
	)

	items, err := svc.ListSelectableModels(context.Background(), &SelectableLive2DModelsRequest{
		Scene:        model.Live2DSceneCompanion,
		IndustryCode: "go",
	})
	if err != nil {
		t.Fatalf("ListSelectableModels returned error: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 selectable models, got %d", len(items))
	}
	if items[0].Key != "db:101" || !items[0].IsRecommended || items[0].MatchType != "industry" {
		t.Fatalf("expected first item to be recommended go model, got %#v", items[0])
	}
	if items[0].ConfigJSON == "" {
		t.Fatalf("expected first item to carry config_json, got %#v", items[0])
	}
	if items[1].Key != "db:102" || items[1].MatchType != "generic" {
		t.Fatalf("expected second item to be generic model, got %#v", items[1])
	}
	if items[2].Key != "db:103" || items[2].MatchType != "other" {
		t.Fatalf("expected third item to be other active model, got %#v", items[2])
	}
}

// TestLive2DServiceListSelectableModelsIncludesFallbackMotions 验证前台切换列表会携带后端回退发现到的动作清单。
func TestLive2DServiceListSelectableModelsIncludesFallbackMotions(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)

	modelDir := filepath.Join(assetsDir, "yumi")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("create model dir: %v", err)
	}
	writeLive2DDirectiveJSONFile(t, filepath.Join(modelDir, "yumi.model3.json"), `{
  "FileReferences": {
    "Expressions": []
  }
}`)
	writeLive2DDirectiveJSONFile(t, filepath.Join(modelDir, "wave.motion3.json"), `{}`)
	writeLive2DDirectiveJSONFile(t, filepath.Join(modelDir, "tear.motion3.json"), `{}`)

	svc := NewLive2DService(
		&mockLive2DModelRepository{
			models: []model.Live2DModel{
				{
					BaseModel: model.BaseModel{ID: 201},
					Name:      "Yumi",
					Scene:     model.Live2DSceneCompanion,
					ModelURL:  "/live2d-assets/yumi/yumi.model3.json",
					IsActive:  true,
				},
			},
		},
		nil,
	)

	items, err := svc.ListSelectableModels(context.Background(), &SelectableLive2DModelsRequest{
		Scene: model.Live2DSceneCompanion,
	})
	if err != nil {
		t.Fatalf("ListSelectableModels returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 selectable model, got %#v", items)
	}
	if len(items[0].Motions) != 2 {
		t.Fatalf("expected fallback motions to be included, got %#v", items[0].Motions)
	}
	if items[0].Motions[0].Group != "auto" {
		t.Fatalf("expected fallback motion group auto, got %#v", items[0].Motions[0])
	}
}

// TestLive2DServiceListSelectableModelsFallsBackToLocalAssets 验证数据库无可用模型时会回退到本地资源目录。
func TestLive2DServiceListSelectableModelsFallsBackToLocalAssets(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)

	modelDir := filepath.Join(assetsDir, "ariu")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("create model dir: %v", err)
	}
	writeLive2DDirectiveJSONFile(t, filepath.Join(modelDir, "ariu.model3.json"), `{
  "FileReferences": {
    "Expressions": []
  }
}`)
	writeLive2DDirectiveJSONFile(t, filepath.Join(modelDir, "intro.motion3.json"), `{}`)
	if err := os.WriteFile(filepath.Join(modelDir, "cover.png"), []byte("png"), 0o644); err != nil {
		t.Fatalf("write thumbnail: %v", err)
	}

	svc := NewLive2DService(nil, nil)

	items, err := svc.ListSelectableModels(context.Background(), &SelectableLive2DModelsRequest{
		Scene: model.Live2DSceneInterview,
	})
	if err != nil {
		t.Fatalf("ListSelectableModels returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one local fallback model, got %#v", items)
	}
	if items[0].Source != "local" || items[0].Key != "local:ariu" {
		t.Fatalf("expected local fallback model, got %#v", items[0])
	}
	if !items[0].IsRecommended || items[0].MatchType != "generic" {
		t.Fatalf("expected local fallback model to be recommended generic item, got %#v", items[0])
	}
}

// TestLive2DServiceGetCurrentModelFallsBackToLocalAssets 验证公开接口会回退返回本地资源模型。
func TestLive2DServiceGetCurrentModelFallsBackToLocalAssets(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)

	modelDir := filepath.Join(assetsDir, "yumi")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("create model dir: %v", err)
	}
	writeLive2DDirectiveJSONFile(t, filepath.Join(modelDir, "yumi.model3.json"), `{
  "FileReferences": {
    "Expressions": []
  }
}`)

	svc := NewLive2DService(nil, nil)

	resp, err := svc.GetCurrentModel(context.Background(), &CurrentLive2DModelRequest{
		Scene: model.Live2DSceneCompanion,
	})
	if err != nil {
		t.Fatalf("GetCurrentModel returned error: %v", err)
	}
	if resp.Source != "local" || resp.ModelURL != "/live2d-assets/yumi/yumi.model3.json" {
		t.Fatalf("expected local fallback current model, got %#v", resp)
	}
}

// TestLive2DServiceGetCurrentModelWithoutConfirmedModels 验证资源目录为空时公开接口返回未找到。
func TestLive2DServiceGetCurrentModelWithoutConfirmedModels(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)

	svc := NewLive2DService(nil, nil)

	resp, err := svc.GetCurrentModel(context.Background(), &CurrentLive2DModelRequest{
		Scene: model.Live2DSceneCompanion,
	})
	if err == nil {
		t.Fatalf("expected not found error, got response %#v", resp)
	}
	businessErr, ok := err.(*common.BusinessError)
	if !ok || businessErr.Code != common.CodeNotFound {
		t.Fatalf("expected not found business error, got %v", err)
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
