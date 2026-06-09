package biz_test

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"makejob/app/admin/internal/biz"
	"makejob/app/admin/internal/data"
	"makejob/app/admin/internal/data/model"
	"makejob/pkg/live2dassets"
)

// TestAdminUseCaseListSelectableLive2DModelsOrdersMatches 验证前台可切换模型列表按行业、通用、其他顺序返回。
func TestAdminUseCaseListSelectableLive2DModelsOrdersMatches(t *testing.T) {
	db := newLive2DUseCaseTestDB(t)
	ctx := context.Background()
	t.Setenv(live2dassets.AssetsDirEnv, t.TempDir())

	industryGo := &model.Industry{Code: "go", Name: "Go"}
	industryJava := &model.Industry{Code: "java", Name: "Java"}
	if err := db.Create(industryGo).Error; err != nil {
		t.Fatalf("create go industry: %v", err)
	}
	if err := db.Create(industryJava).Error; err != nil {
		t.Fatalf("create java industry: %v", err)
	}

	if err := db.Create(&model.Live2DModel{
		Name:     "Go 专属",
		Scene:    "companion",
		ModelURL: "/live2d-assets/go/model.model3.json",
		IsActive: true,
		IndustryID: func() *uint {
			value := industryGo.ID
			return &value
		}(),
	}).Error; err != nil {
		t.Fatalf("create go live2d model: %v", err)
	}
	if err := db.Create(&model.Live2DModel{
		Name:     "通用模型",
		Scene:    "companion",
		ModelURL: "/live2d-assets/generic/model.model3.json",
		IsActive: true,
	}).Error; err != nil {
		t.Fatalf("create generic live2d model: %v", err)
	}
	if err := db.Create(&model.Live2DModel{
		Name:     "Java 模型",
		Scene:    "companion",
		ModelURL: "/live2d-assets/java/model.model3.json",
		IsActive: true,
		IndustryID: func() *uint {
			value := industryJava.ID
			return &value
		}(),
	}).Error; err != nil {
		t.Fatalf("create java live2d model: %v", err)
	}

	uc := biz.NewAdminUseCase(data.NewAdminRepo(db), nil, nil, nil, nil)
	models, err := uc.ListSelectableLive2DModels(ctx, "companion", "go")
	if err != nil {
		t.Fatalf("ListSelectableLive2DModels returned error: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("expected 3 selectable models, got %d", len(models))
	}
	if models[0].Name != "Go 专属" || models[0].MatchType != "industry" || !models[0].IsRecommended {
		t.Fatalf("unexpected first selectable model: %#v", models[0])
	}
	if models[1].Name != "通用模型" || models[1].MatchType != "generic" {
		t.Fatalf("unexpected second selectable model: %#v", models[1])
	}
	if models[2].Name != "Java 模型" || models[2].MatchType != "other" {
		t.Fatalf("unexpected third selectable model: %#v", models[2])
	}
}

// TestAdminUseCaseGetCurrentLive2DModelFallsBackToGeneric 验证当前模型查询会优先回退到通用激活模型。
func TestAdminUseCaseGetCurrentLive2DModelFallsBackToGeneric(t *testing.T) {
	db := newLive2DUseCaseTestDB(t)
	ctx := context.Background()
	t.Setenv(live2dassets.AssetsDirEnv, t.TempDir())

	industryGo := &model.Industry{Code: "go", Name: "Go"}
	if err := db.Create(industryGo).Error; err != nil {
		t.Fatalf("create go industry: %v", err)
	}
	if err := db.Create(&model.Live2DModel{
		Name:       "通用模型",
		Scene:      "interview",
		ModelURL:   "/live2d-assets/generic/interview.model3.json",
		ConfigJSON: `{"background_image_url":"/live2d-assets/backgrounds/stage.webp"}`,
		IsActive:   true,
	}).Error; err != nil {
		t.Fatalf("create generic live2d model: %v", err)
	}

	uc := biz.NewAdminUseCase(data.NewAdminRepo(db), nil, nil, nil, nil)
	currentModel, err := uc.GetCurrentLive2DModel(ctx, "interview", "go")
	if err != nil {
		t.Fatalf("GetCurrentLive2DModel returned error: %v", err)
	}
	if currentModel.Name != "通用模型" || currentModel.Source != "database" {
		t.Fatalf("unexpected current model: %#v", currentModel)
	}
	if currentModel.IndustryCode != "" {
		t.Fatalf("expected generic model to clear industry code, got %s", currentModel.IndustryCode)
	}
}

// TestAdminUseCaseImportLive2DPackageCreatesPendingModel 验证导入 ZIP 包后会自动生成未激活的待确认模型记录。
func TestAdminUseCaseImportLive2DPackageCreatesPendingModel(t *testing.T) {
	db := newLive2DUseCaseTestDB(t)
	ctx := context.Background()
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)

	uc := biz.NewAdminUseCase(data.NewAdminRepo(db), nil, nil, nil, nil)
	resp, err := uc.ImportLive2DPackage(ctx, "Ariu.zip", buildLive2DZipPackage(t, map[string]string{
		"Ariu.model3.json": "{}",
		"Ariu.png":         "image",
	}))
	if err != nil {
		t.Fatalf("ImportLive2DPackage returned error: %v", err)
	}
	if !resp.Created || resp.ModelID == 0 || resp.IsActive {
		t.Fatalf("unexpected import result: %#v", resp)
	}

	models, err := uc.ListManagedLive2DModels(ctx)
	if err != nil {
		t.Fatalf("ListManagedLive2DModels returned error: %v", err)
	}
	if len(models) != 1 || models[0].Scene != "companion" || models[0].IsActive {
		t.Fatalf("unexpected managed model after import: len=%d model=%+v", len(models), models[0])
	}
}

// TestAdminUseCaseDeleteManagedLive2DModelRemovesAssetDir 验证删除唯一受管模型后会同步清理资源目录，避免再次被自动补回。
func TestAdminUseCaseDeleteManagedLive2DModelRemovesAssetDir(t *testing.T) {
	db := newLive2DUseCaseTestDB(t)
	ctx := context.Background()
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)

	uc := biz.NewAdminUseCase(data.NewAdminRepo(db), nil, nil, nil, nil)
	importResult, err := uc.ImportLive2DPackage(ctx, "Ariu.zip", buildLive2DZipPackage(t, map[string]string{
		"Ariu.model3.json": "{}",
		"Ariu.png":         "image",
	}))
	if err != nil {
		t.Fatalf("ImportLive2DPackage returned error: %v", err)
	}

	if err := uc.DeleteManagedLive2DModel(ctx, importResult.ModelID); err != nil {
		t.Fatalf("DeleteManagedLive2DModel returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(assetsDir, importResult.AssetDir)); !os.IsNotExist(err) {
		t.Fatalf("expected asset dir to be removed, got err=%v", err)
	}

	models, err := uc.ListManagedLive2DModels(ctx)
	if err != nil {
		t.Fatalf("ListManagedLive2DModels returned error: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("expected deleted model not to be auto-synced back, got %#v", models)
	}
}

// newLive2DUseCaseTestDB 创建 Live2D 用例测试使用的最小 SQLite 数据库。
func newLive2DUseCaseTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(&model.Industry{}, &model.Live2DModel{}); err != nil {
		t.Fatalf("auto migrate live2d tables: %v", err)
	}
	return db
}

// buildLive2DZipPackage 生成测试用的最小 Live2D ZIP 包。
func buildLive2DZipPackage(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entryWriter, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := entryWriter.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buffer.Bytes()
}
