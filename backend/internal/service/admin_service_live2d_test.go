package service

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"makejob-backend/internal/live2dassets"
	"makejob-backend/internal/model"
)

// TestAdminServiceListLive2DModelsSyncDiscoveredAssets 验证后台列表会自动补登记本地未入库模型。
func TestAdminServiceListLive2DModelsSyncDiscoveredAssets(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)
	writeAdminServiceTestModel(t, assetsDir, "Ariu", "Ariu.model3.json", "Ariu.png")

	repo := &adminServiceLive2DRepoStub{}
	svc := &adminService{
		live2DRepo: repo,
	}

	models, err := svc.ListLive2DModels(context.Background())
	if err != nil {
		t.Fatalf("ListLive2DModels returned error: %v", err)
	}

	if len(models) != 1 {
		t.Fatalf("expected 1 synced model, got %d", len(models))
	}
	if models[0].Name != "Ariu" {
		t.Fatalf("expected synced model name Ariu, got %s", models[0].Name)
	}
	if models[0].Scene != model.Live2DSceneCompanion {
		t.Fatalf("expected default companion scene, got %s", models[0].Scene)
	}
	if models[0].IsActive {
		t.Fatalf("expected synced model to be inactive by default")
	}
}

// TestAdminServiceListLive2DModelsSkipExistingAsset 验证已入库资源不会因自动扫描重复创建记录。
func TestAdminServiceListLive2DModelsSkipExistingAsset(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)
	writeAdminServiceTestModel(t, assetsDir, "Ariu", "Ariu.model3.json", "Ariu.png")

	repo := &adminServiceLive2DRepoStub{
		models: []model.Live2DModel{
			{
				BaseModel:    model.BaseModel{ID: 7},
				Name:         "Ariu",
				Scene:        model.Live2DSceneCompanion,
				ModelURL:     "/live2d-assets/Ariu/Ariu.model3.json",
				ThumbnailURL: "/live2d-assets/Ariu/Ariu.png",
				IsActive:     false,
			},
		},
	}
	svc := &adminService{
		live2DRepo: repo,
	}

	models, err := svc.ListLive2DModels(context.Background())
	if err != nil {
		t.Fatalf("ListLive2DModels returned error: %v", err)
	}

	if len(models) != 1 {
		t.Fatalf("expected existing model to remain singular, got %d", len(models))
	}
	if repo.createCount != 0 {
		t.Fatalf("expected no duplicate create, got %d", repo.createCount)
	}
}

// TestAdminServiceImportLive2DPackageCreatesPendingModel 验证 ZIP 导入后会自动生成待确认后台记录。
func TestAdminServiceImportLive2DPackageCreatesPendingModel(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)

	repo := &adminServiceLive2DRepoStub{}
	svc := &adminService{
		live2DRepo: repo,
	}

	resp, err := svc.ImportLive2DPackage(context.Background(), "Ariu.zip", buildAdminServiceTestZipPackage(t, map[string]string{
		"Ariu.model3.json": "{}",
		"Ariu.png":         "image",
	}))
	if err != nil {
		t.Fatalf("ImportLive2DPackage returned error: %v", err)
	}

	if !resp.Created {
		t.Fatalf("expected imported model to be newly created")
	}
	if resp.IsActive {
		t.Fatalf("expected imported model to be inactive by default")
	}
	if resp.ModelID == 0 {
		t.Fatalf("expected imported model id to be returned")
	}
	if repo.createCount != 1 {
		t.Fatalf("expected exactly one created model, got %d", repo.createCount)
	}
	if len(repo.models) != 1 || repo.models[0].Scene != model.Live2DSceneCompanion {
		t.Fatalf("expected pending companion model record, got %#v", repo.models)
	}
}

// TestAdminServiceImportLive2DPackageReuseExistingModel 验证重复导入同一模型资源时不会重复落库。
func TestAdminServiceImportLive2DPackageReuseExistingModel(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)

	repo := &adminServiceLive2DRepoStub{
		models: []model.Live2DModel{
			{
				BaseModel:    model.BaseModel{ID: 11},
				Name:         "Ariu",
				Scene:        model.Live2DSceneCompanion,
				ModelURL:     "/live2d-assets/Ariu/Ariu.model3.json",
				ThumbnailURL: "/live2d-assets/Ariu/Ariu.png",
				IsActive:     false,
			},
		},
	}
	svc := &adminService{
		live2DRepo: repo,
	}

	resp, err := svc.ImportLive2DPackage(context.Background(), "Ariu.zip", buildAdminServiceTestZipPackage(t, map[string]string{
		"Ariu.model3.json": "{}",
		"Ariu.png":         "image",
	}))
	if err != nil {
		t.Fatalf("ImportLive2DPackage returned error: %v", err)
	}

	if resp.Created {
		t.Fatalf("expected existing model to be reused")
	}
	if resp.ModelID != 11 {
		t.Fatalf("expected existing model id 11, got %d", resp.ModelID)
	}
	if repo.createCount != 0 {
		t.Fatalf("expected no duplicate create, got %d", repo.createCount)
	}
}

// TestAdminServiceDeleteLive2DModelRemovesManagedAssets 验证删除模型时会同步移除本地资源目录，避免再次被自动补回。
func TestAdminServiceDeleteLive2DModelRemovesManagedAssets(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)
	writeAdminServiceTestModel(t, assetsDir, "Ariu", "Ariu.model3.json", "Ariu.png")

	repo := &adminServiceLive2DRepoStub{
		models: []model.Live2DModel{
			{
				BaseModel:    model.BaseModel{ID: 21},
				Name:         "Ariu",
				Scene:        model.Live2DSceneCompanion,
				ModelURL:     "/live2d-assets/Ariu/Ariu.model3.json",
				ThumbnailURL: "/live2d-assets/Ariu/Ariu.png",
				IsActive:     false,
			},
		},
	}
	svc := &adminService{
		live2DRepo: repo,
	}

	if err := svc.DeleteLive2DModel(context.Background(), 21); err != nil {
		t.Fatalf("DeleteLive2DModel returned error: %v", err)
	}

	if len(repo.models) != 0 {
		t.Fatalf("expected model record to be deleted, got %#v", repo.models)
	}
	if _, err := os.Stat(filepath.Join(assetsDir, "Ariu")); !os.IsNotExist(err) {
		t.Fatalf("expected managed asset dir to be removed, got err=%v", err)
	}

	models, err := svc.ListLive2DModels(context.Background())
	if err != nil {
		t.Fatalf("ListLive2DModels returned error after delete: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("expected deleted model not to be auto-synced back, got %#v", models)
	}
}

// TestAdminServiceDeleteLive2DModelKeepsSharedAssets 验证共享同一资源目录时删除单条记录不会误删模型文件。
func TestAdminServiceDeleteLive2DModelKeepsSharedAssets(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)
	writeAdminServiceTestModel(t, assetsDir, "Ariu", "Ariu.model3.json", "Ariu.png")

	repo := &adminServiceLive2DRepoStub{
		models: []model.Live2DModel{
			{
				BaseModel:    model.BaseModel{ID: 31},
				Name:         "Ariu-1",
				Scene:        model.Live2DSceneCompanion,
				ModelURL:     "/live2d-assets/Ariu/Ariu.model3.json",
				ThumbnailURL: "/live2d-assets/Ariu/Ariu.png",
				IsActive:     false,
			},
			{
				BaseModel:    model.BaseModel{ID: 32},
				Name:         "Ariu-2",
				Scene:        model.Live2DSceneInterview,
				ModelURL:     "/live2d-assets/Ariu/Ariu.model3.json",
				ThumbnailURL: "/live2d-assets/Ariu/Ariu.png",
				IsActive:     false,
			},
		},
	}
	svc := &adminService{
		live2DRepo: repo,
	}

	if err := svc.DeleteLive2DModel(context.Background(), 31); err != nil {
		t.Fatalf("DeleteLive2DModel returned error: %v", err)
	}

	if len(repo.models) != 1 || repo.models[0].ID != 32 {
		t.Fatalf("expected one shared model to remain, got %#v", repo.models)
	}
	if _, err := os.Stat(filepath.Join(assetsDir, "Ariu")); err != nil {
		t.Fatalf("expected shared asset dir to remain, got err=%v", err)
	}
}

// writeAdminServiceTestModel 写入一个可被后台自动识别的最小模型目录。
func writeAdminServiceTestModel(t *testing.T, assetsDir string, dirName string, modelFileName string, thumbnailFileName string) {
	t.Helper()

	modelDir := filepath.Join(assetsDir, dirName)
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("create model dir %s: %v", dirName, err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, modelFileName), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write model file %s: %v", modelFileName, err)
	}
	if thumbnailFileName != "" {
		if err := os.WriteFile(filepath.Join(modelDir, thumbnailFileName), []byte("image"), 0o644); err != nil {
			t.Fatalf("write thumbnail file %s: %v", thumbnailFileName, err)
		}
	}
}

// buildAdminServiceTestZipPackage 生成一个最小可导入 ZIP 包，供后台导入测试复用。
func buildAdminServiceTestZipPackage(t *testing.T, files map[string]string) []byte {
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

// adminServiceLive2DRepoStub 模拟后台 Live2D 模型仓库。
type adminServiceLive2DRepoStub struct {
	models      []model.Live2DModel
	createCount int
}

// List 返回当前仓库中的模型列表。
func (s *adminServiceLive2DRepoStub) List(context.Context) ([]model.Live2DModel, error) {
	return append([]model.Live2DModel(nil), s.models...), nil
}

// GetByID 按 ID 返回模型记录。
func (s *adminServiceLive2DRepoStub) GetByID(_ context.Context, id uint) (*model.Live2DModel, error) {
	for i := range s.models {
		if s.models[i].ID == id {
			modelCopy := s.models[i]
			return &modelCopy, nil
		}
	}
	return nil, nil
}

// Create 记录新建模型并模拟数据库分配主键。
func (s *adminServiceLive2DRepoStub) Create(_ context.Context, live2d *model.Live2DModel) error {
	s.createCount++
	modelCopy := *live2d
	if modelCopy.ID == 0 {
		modelCopy.ID = uint(len(s.models) + 1)
	}
	*live2d = modelCopy
	s.models = append(s.models, modelCopy)
	return nil
}

// Update 在当前测试中不需要具体行为。
func (s *adminServiceLive2DRepoStub) Update(context.Context, *model.Live2DModel) error {
	return nil
}

// Delete 删除指定模型记录，模拟后台仓库行为。
func (s *adminServiceLive2DRepoStub) Delete(_ context.Context, id uint) error {
	filtered := make([]model.Live2DModel, 0, len(s.models))
	for _, currentModel := range s.models {
		if currentModel.ID == id {
			continue
		}
		filtered = append(filtered, currentModel)
	}
	s.models = filtered
	return nil
}
