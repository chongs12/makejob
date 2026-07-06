package data

import (
	"testing"

	"makejob/app/companion/internal/biz"
)

// TestVolcengineASRProvider_GetSupportedEngines 验证火山引擎 ASR 支持的引擎列表。
func TestVolcengineASRProvider_GetSupportedEngines(t *testing.T) {
	provider := NewVolcengineASRProvider("test-app-id", "test-token", "", "")

	engines := provider.GetSupportedEngines()
	if len(engines) == 0 {
		t.Fatal("expected non-empty engines list")
	}

	found := false
	for _, e := range engines {
		if e == "volcengine" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'volcengine' in engines, got %v", engines)
	}
}

// TestVolcengineASRProvider_EmptyAudio 验证空音频输入返回错误。
func TestVolcengineASRProvider_EmptyAudio(t *testing.T) {
	provider := NewVolcengineASRProvider("test-app-id", "test-token", "", "")

	_, err := provider.Recognize(t.Context(), biz.ASRRequest{
		AudioData:  []byte{},
		Format:     "pcm",
		SampleRate: 16000,
		Language:   "zh-CN",
	})
	if err == nil {
		t.Fatal("expected error for empty audio")
	}
}

// TestVolcengineASRProvider_NilConfig 验证空配置创建的 provider 不会 panic。
func TestVolcengineASRProvider_NilConfig(t *testing.T) {
	// 不应该 panic
	_ = NewVolcengineASRProvider("", "", "", "")
}
