package live2dassets

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestImportZipSuccess 验证导入模型包后会返回可访问的模型地址和缩略图。
func TestImportZipSuccess(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(AssetsDirEnv, assetsDir)

	rawZip := buildZipArchive(t, map[string]string{
		"小恶魔.model3.json":         `{}`,
		"textures/texture_00.png": "fake-image",
	})

	importedPackage, err := ImportZip("小恶魔.zip", rawZip)
	if err != nil {
		t.Fatalf("expected import success, got error: %v", err)
	}

	if importedPackage.Name != "小恶魔" {
		t.Fatalf("expected imported name 小恶魔, got %s", importedPackage.Name)
	}
	if !strings.HasSuffix(importedPackage.ModelURL, "/小恶魔/小恶魔.model3.json") {
		t.Fatalf("expected model url suffix /小恶魔/小恶魔.model3.json, got %s", importedPackage.ModelURL)
	}
	if !strings.HasSuffix(importedPackage.ThumbnailURL, "/小恶魔/textures/texture_00.png") {
		t.Fatalf("expected thumbnail url suffix /小恶魔/textures/texture_00.png, got %s", importedPackage.ThumbnailURL)
	}

	modelPath := filepath.Join(assetsDir, filepath.FromSlash(importedPackage.ModelPath))
	if _, err := os.Stat(modelPath); err != nil {
		t.Fatalf("expected extracted model file, got stat error: %v", err)
	}
}

// TestImportZipRejectsTraversal 验证导入时会拒绝包含目录穿越的压缩包。
func TestImportZipRejectsTraversal(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(AssetsDirEnv, assetsDir)

	rawZip := buildZipArchive(t, map[string]string{
		"../escape.model3.json": `{}`,
	})

	if _, err := ImportZip("escape.zip", rawZip); err == nil {
		t.Fatal("expected path traversal zip to be rejected")
	}
}

// TestDiscoverLocalModels 验证本地资源目录扫描会返回可访问的模型和缩略图地址。
func TestDiscoverLocalModels(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(AssetsDirEnv, assetsDir)

	modelDir := filepath.Join(assetsDir, "yumi")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("create model dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "yumi.model3.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write model file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "yumi.webp"), []byte("image"), 0o644); err != nil {
		t.Fatalf("write thumbnail file: %v", err)
	}

	models, err := DiscoverLocalModels()
	if err != nil {
		t.Fatalf("DiscoverLocalModels returned error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 discovered model, got %d", len(models))
	}
	if models[0].Name != "yumi" {
		t.Fatalf("expected model name yumi, got %s", models[0].Name)
	}
	if !strings.HasSuffix(models[0].ModelURL, "/yumi/yumi.model3.json") {
		t.Fatalf("unexpected model url: %s", models[0].ModelURL)
	}
	if !strings.HasSuffix(models[0].ThumbnailURL, "/yumi/yumi.webp") {
		t.Fatalf("unexpected thumbnail url: %s", models[0].ThumbnailURL)
	}
}

// TestImportBackgroundImage 验证上传舞台背景图后会返回可直接访问的静态地址。
func TestImportBackgroundImage(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(AssetsDirEnv, assetsDir)

	importedBackground, err := ImportBackgroundImage("stage-cover.png", []byte("fake-image"))
	if err != nil {
		t.Fatalf("expected import success, got error: %v", err)
	}

	if importedBackground.FileName != "stage-cover.png" {
		t.Fatalf("expected file name stage-cover.png, got %s", importedBackground.FileName)
	}
	if !strings.HasSuffix(importedBackground.AssetURL, "/backgrounds/stage-cover.png") {
		t.Fatalf("unexpected background asset url: %s", importedBackground.AssetURL)
	}

	backgroundPath := filepath.Join(assetsDir, filepath.FromSlash(importedBackground.AssetPath))
	if _, err := os.Stat(backgroundPath); err != nil {
		t.Fatalf("expected extracted background file, got stat error: %v", err)
	}
}

// TestManagedModelAssetDirFromURL 验证模型 URL 可以正确解析为受管资源目录。
func TestManagedModelAssetDirFromURL(t *testing.T) {
	if dir := ManagedModelAssetDirFromURL("/live2d-assets/yumi/yumi.model3.json"); dir != "yumi" {
		t.Fatalf("expected asset dir yumi, got %s", dir)
	}
	if dir := ManagedModelAssetDirFromURL("/live2d-assets/backgrounds/stage-cover.png"); dir != "" {
		t.Fatalf("expected backgrounds to be ignored, got %s", dir)
	}
	if dir := ManagedModelAssetDirFromURL("https://example.com/yumi.model3.json"); dir != "" {
		t.Fatalf("expected external url to be ignored, got %s", dir)
	}
}

// TestDeleteManagedModelAssetDir 验证删除受管模型目录时不会误删其他目录。
func TestDeleteManagedModelAssetDir(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(AssetsDirEnv, assetsDir)

	modelDir := filepath.Join(assetsDir, "yumi")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("create model dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "yumi.model3.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write model file: %v", err)
	}

	if err := DeleteManagedModelAssetDir("yumi"); err != nil {
		t.Fatalf("DeleteManagedModelAssetDir returned error: %v", err)
	}
	if _, err := os.Stat(modelDir); !os.IsNotExist(err) {
		t.Fatalf("expected model dir to be removed, got err=%v", err)
	}
}

// buildZipArchive 构造测试用的 ZIP 数据。
func buildZipArchive(t *testing.T, files map[string]string) []byte {
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
