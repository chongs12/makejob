package service

import (
	"context"
	"testing"

	companionv1 "makejob/api/makejob/companion/v1"
	"makejob/app/companion/internal/biz"
)

// TestRecognizeSpeech_MockProvider 验证 RecognizeSpeech 端到端流程（通过工厂动态创建）。
func TestRecognizeSpeech_MockProvider(t *testing.T) {
	uc := biz.NewCompanionUseCase(
		nil, nil, nil, "",
		biz.WithASRProviderFactory(func(cfg *biz.ASRConfig) (biz.ASRProvider, error) {
			return biz.NewMockASRProvider(), nil
		}),
		// 提供一个简单的 config repo，返回一个 mock 配置
		biz.WithASRConfigRepo(&mockASRConfigRepo{}),
	)

	svc := NewCompanionService(uc)

	resp, err := svc.RecognizeSpeech(context.Background(), &companionv1.RecognizeSpeechRequest{
		AudioData:  []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
		Format:     "pcm",
		SampleRate: 16000,
		Language:   "zh-CN",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.GetText() == "" {
		t.Fatal("expected non-empty text")
	}
	if resp.GetConfidence() <= 0 {
		t.Fatalf("expected positive confidence, got %f", resp.GetConfidence())
	}
	if resp.GetDuration() <= 0 {
		t.Fatalf("expected positive duration, got %f", resp.GetDuration())
	}
}

// TestRecognizeSpeech_EmptyAudio 验证空音频返回错误。
func TestRecognizeSpeech_EmptyAudio(t *testing.T) {
	uc := biz.NewCompanionUseCase(nil, nil, nil, "")

	svc := NewCompanionService(uc)

	_, err := svc.RecognizeSpeech(context.Background(), &companionv1.RecognizeSpeechRequest{
		AudioData:  []byte{},
		Format:     "pcm",
		SampleRate: 16000,
		Language:   "zh-CN",
	})
	if err == nil {
		t.Fatal("expected error for empty audio")
	}
}

// TestRecognizeSpeech_NoFactory 验证未配置 ASR 工厂时返回错误。
func TestRecognizeSpeech_NoFactory(t *testing.T) {
	uc := biz.NewCompanionUseCase(nil, nil, nil, "")

	svc := NewCompanionService(uc)

	_, err := svc.RecognizeSpeech(context.Background(), &companionv1.RecognizeSpeechRequest{
		AudioData:  []byte{0x00, 0x01},
		Format:     "pcm",
		SampleRate: 16000,
		Language:   "zh-CN",
	})
	if err == nil {
		t.Fatal("expected error when ASR not configured")
	}
}

// mockASRConfigRepo 测试用的 ASR 配置仓库，返回一条 mock 配置。
type mockASRConfigRepo struct{}

func (r *mockASRConfigRepo) GetByID(_ context.Context, _ uint) (*biz.ASRConfig, error) {
	return &biz.ASRConfig{ID: 1, Name: "mock", Engine: "mock", IsActive: true}, nil
}

func (r *mockASRConfigRepo) List(_ context.Context) ([]biz.ASRConfig, error) {
	return []biz.ASRConfig{{ID: 1, Name: "mock", Engine: "mock", IsActive: true}}, nil
}

func (r *mockASRConfigRepo) Create(_ context.Context, _ *biz.ASRConfig) error { return nil }
func (r *mockASRConfigRepo) Update(_ context.Context, _ *biz.ASRConfig) error { return nil }
func (r *mockASRConfigRepo) Delete(_ context.Context, _ uint) error          { return nil }
