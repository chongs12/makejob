package biz

import (
	"context"
	"testing"
)

// TestMockASRProvider_Recognize 验证 Mock ASR 供应商能返回固定识别结果。
func TestMockASRProvider_Recognize(t *testing.T) {
	provider := NewMockASRProvider()

	result, err := provider.Recognize(context.Background(), ASRRequest{
		AudioData:  []byte{0x00, 0x01, 0x02, 0x03},
		Format:     "pcm",
		SampleRate: 16000,
		Language:   "zh-CN",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Text == "" {
		t.Fatal("expected non-empty text")
	}
	if result.Confidence <= 0 || result.Confidence > 1 {
		t.Fatalf("expected confidence in (0,1], got %f", result.Confidence)
	}
	if result.Duration <= 0 {
		t.Fatalf("expected positive duration, got %f", result.Duration)
	}
}

// TestMockASRProvider_EmptyAudio 验证空音频输入返回错误。
func TestMockASRProvider_EmptyAudio(t *testing.T) {
	provider := NewMockASRProvider()

	_, err := provider.Recognize(context.Background(), ASRRequest{
		AudioData:  []byte{},
		Format:     "pcm",
		SampleRate: 16000,
		Language:   "zh-CN",
	})
	if err == nil {
		t.Fatal("expected error for empty audio")
	}
}

// TestMockASRProvider_GetSupportedEngines 验证 Mock 支持的引擎列表。
func TestMockASRProvider_GetSupportedEngines(t *testing.T) {
	provider := NewMockASRProvider()

	engines := provider.GetSupportedEngines()
	if len(engines) == 0 {
		t.Fatal("expected non-empty engines list")
	}
}
