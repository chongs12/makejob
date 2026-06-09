package live2dassets

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestImportZipReturnsDetectedModel 验证导入 ZIP 后会识别主模型与缩略图地址。
func TestImportZipReturnsDetectedModel(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(AssetsDirEnv, assetsDir)

	resp, err := ImportZip("Ariu.zip", buildTestZipPackage(t, map[string]string{
		"Ariu.model3.json": "{}",
		"Ariu.png":         "image",
	}))
	if err != nil {
		t.Fatalf("ImportZip returned error: %v", err)
	}
	if resp.AssetDir != "ariu" {
		t.Fatalf("expected asset dir ariu, got %s", resp.AssetDir)
	}
	if resp.ModelURL != "/live2d-assets/ariu/Ariu.model3.json" {
		t.Fatalf("unexpected model url: %s", resp.ModelURL)
	}
	if resp.ThumbnailURL != "/live2d-assets/ariu/Ariu.png" {
		t.Fatalf("unexpected thumbnail url: %s", resp.ThumbnailURL)
	}
}

// TestImportBackgroundImageAllocatesUniqueName 验证背景图重复导入时会自动分配不冲突文件名。
func TestImportBackgroundImageAllocatesUniqueName(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(AssetsDirEnv, assetsDir)

	first, err := ImportBackgroundImage("stage.png", []byte("first"))
	if err != nil {
		t.Fatalf("ImportBackgroundImage returned error: %v", err)
	}
	second, err := ImportBackgroundImage("stage.png", []byte("second"))
	if err != nil {
		t.Fatalf("ImportBackgroundImage returned error: %v", err)
	}
	if first.FileName == second.FileName {
		t.Fatalf("expected distinct background filenames, got %s", first.FileName)
	}
}

// TestDiscoverLocalModelsListsImportedPackage 验证本地受管目录中的模型可以被自动发现。
func TestDiscoverLocalModelsListsImportedPackage(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(AssetsDirEnv, assetsDir)

	modelDir := filepath.Join(assetsDir, "yumi")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("create model dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "yumi.model3.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write model file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "cover.png"), []byte("image"), 0o644); err != nil {
		t.Fatalf("write thumbnail file: %v", err)
	}

	models, err := DiscoverLocalModels()
	if err != nil {
		t.Fatalf("DiscoverLocalModels returned error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 discovered model, got %d", len(models))
	}
	if models[0].ModelURL != "/live2d-assets/yumi/yumi.model3.json" {
		t.Fatalf("unexpected model url: %s", models[0].ModelURL)
	}
}

// TestManagedModelAssetDirFromURL 验证受管模型 URL 可以稳定解析资源目录名。
func TestManagedModelAssetDirFromURL(t *testing.T) {
	if dir := ManagedModelAssetDirFromURL("/live2d-assets/yumi/yumi.model3.json"); dir != "yumi" {
		t.Fatalf("expected asset dir yumi, got %s", dir)
	}
	if dir := ManagedModelAssetDirFromURL("/live2d-assets/backgrounds/stage-cover.png"); dir != "" {
		t.Fatalf("expected background asset dir to be ignored, got %s", dir)
	}
}

// buildTestZipPackage 生成测试用的最小 ZIP 包。
func buildTestZipPackage(t *testing.T, files map[string]string) []byte {
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
