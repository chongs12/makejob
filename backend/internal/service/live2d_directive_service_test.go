package service

import (
	"os"
	"path/filepath"
	"testing"

	"makejob-backend/internal/live2dassets"
	"makejob-backend/internal/model"
)

// TestBuildLive2DManifestFromModelIncludesMotions 验证 manifest 会从 model3.json 中解析动作白名单。
func TestBuildLive2DManifestFromModelIncludesMotions(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)

	modelDir := filepath.Join(assetsDir, "yumi")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("create model dir: %v", err)
	}
	writeLive2DDirectiveJSONFile(t, filepath.Join(modelDir, "yumi.model3.json"), `{
  "FileReferences": {
    "Expressions": [{ "Name": "happy", "File": "happy.exp3.json" }],
    "Motions": {
      "TapBody": [
        { "File": "wave.motion3.json" },
        { "File": "tear.motion3.json" }
      ]
    }
  }
}`)
	writeLive2DDirectiveJSONFile(t, filepath.Join(modelDir, "yumi.cdi3.json"), `{
  "Parameters": [{ "Id": "ParamMouthOpenY", "Name": "Mouth Open" }]
}`)

	manifest, err := buildLive2DManifestFromModel(newTestManagedLive2DModel("Yumi", "/live2d-assets/yumi/yumi.model3.json"))
	if err != nil {
		t.Fatalf("buildLive2DManifestFromModel returned error: %v", err)
	}

	if len(manifest.Motions) != 2 {
		t.Fatalf("expected 2 motions, got %d", len(manifest.Motions))
	}
	if manifest.Motions[0].Group != "tapbody" {
		t.Fatalf("expected normalized motion group tapbody, got %#v", manifest.Motions[0])
	}
	if manifest.Motions[0].Key == "" || manifest.Motions[0].File == "" {
		t.Fatalf("expected motion key and file to be populated, got %#v", manifest.Motions[0])
	}
}

// TestBuildLive2DManifestFromModelFallsBackToDirectoryMotions 验证未声明 Motions 时会回退扫描目录中的 motion3 文件。
func TestBuildLive2DManifestFromModelFallsBackToDirectoryMotions(t *testing.T) {
	assetsDir := t.TempDir()
	t.Setenv(live2dassets.AssetsDirEnv, assetsDir)

	modelDir := filepath.Join(assetsDir, "huohuo")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("create model dir: %v", err)
	}
	writeLive2DDirectiveJSONFile(t, filepath.Join(modelDir, "huohuo.model3.json"), `{
  "FileReferences": {
    "Expressions": []
  }
}`)
	writeLive2DDirectiveJSONFile(t, filepath.Join(modelDir, "haoqi.motion3.json"), `{}`)
	writeLive2DDirectiveJSONFile(t, filepath.Join(modelDir, "keshui.motion3.json"), `{}`)

	manifest, err := buildLive2DManifestFromModel(newTestManagedLive2DModel("Huohuo", "/live2d-assets/huohuo/huohuo.model3.json"))
	if err != nil {
		t.Fatalf("buildLive2DManifestFromModel returned error: %v", err)
	}

	if len(manifest.Motions) != 2 {
		t.Fatalf("expected fallback-discovered motions, got %#v", manifest.Motions)
	}
	for _, item := range manifest.Motions {
		if item.Group != "auto" {
			t.Fatalf("expected fallback motion group auto, got %#v", item)
		}
	}
}

// TestNormalizeLive2DMotionKeyKeepsStableToken 验证动作键规整逻辑可生成稳定 token。
func TestNormalizeLive2DMotionKeyKeepsStableToken(t *testing.T) {
	got := normalizeManifestMotionKey("TapBody Wave.motion3.json", 0)
	if got != "tapbody_wave_motion3_json" && got != "tapbody_wave" {
		t.Fatalf("unexpected normalized key: %s", got)
	}
}

// writeLive2DDirectiveJSONFile 以 UTF-8 写入测试所需的 JSON 文件。
func writeLive2DDirectiveJSONFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

// newTestManagedLive2DModel 构造一个最小受管 Live2D 模型记录供 manifest 测试复用。
func newTestManagedLive2DModel(name string, modelURL string) model.Live2DModel {
	return model.Live2DModel{
		Name:     name,
		Scene:    "companion",
		ModelURL: modelURL,
	}
}
