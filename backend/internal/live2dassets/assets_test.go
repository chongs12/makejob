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
