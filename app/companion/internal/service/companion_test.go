package service

import (
	"context"
	"testing"

	companionv1 "makejob/api/makejob/companion/v1"
	"makejob/app/companion/internal/biz"
)

// companionRepoStub 提供最小仓库桩，满足用例构造依赖。
type companionRepoStub struct{}

// GetSession 返回空会话桩。
func (s *companionRepoStub) GetSession(ctx context.Context, userID uint64) (*biz.CompanionSession, error) {
	return nil, nil
}

// CreateOrUpdate 提供空实现桩。
func (s *companionRepoStub) CreateOrUpdate(ctx context.Context, session *biz.CompanionSession) error {
	return nil
}

// companionTTSStub 提供固定的语音合成结果。
type companionTTSStub struct{}

// Synthesize 返回固定音频数据和兼容 URL。
func (s *companionTTSStub) Synthesize(ctx context.Context, text, voice string) (*biz.TTSAudio, error) {
	return &biz.TTSAudio{
		AudioData: []byte("binary-mp3"),
		AudioURL:  "data:audio/mp3;base64,YmluYXJ5LW1wMw==",
	}, nil
}

// TestCompanionServiceSynthesizeSpeech 验证服务层会同时返回音频字节和兼容 URL。
func TestCompanionServiceSynthesizeSpeech(t *testing.T) {
	uc := biz.NewCompanionUseCase(&companionRepoStub{}, nil, &companionTTSStub{}, "test-voice")
	svc := NewCompanionService(uc)

	resp, err := svc.SynthesizeSpeech(context.Background(), &companionv1.SynthesizeSpeechRequest{
		Text:  "你好",
		Voice: "test-voice",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(resp.GetAudioData()) != "binary-mp3" {
		t.Fatalf("expected audio_data to be binary-mp3, got %q", string(resp.GetAudioData()))
	}
	if resp.GetAudioUrl() == "" {
		t.Fatal("expected compatibility audio_url to be populated")
	}
}
