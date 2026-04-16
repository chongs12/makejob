package mock

import (
	"context"
	"fmt"
	"makejob-backend/internal/tts"
)

// MockTTSProvider Mock TTS实现
type MockTTSProvider struct {
	voices []tts.Voice
}

// NewMockTTSProvider 创建Mock TTS Provider
func NewMockTTSProvider() *MockTTSProvider {
	return &MockTTSProvider{
		voices: []tts.Voice{
			// 面试场景专业音色
			{
				ID:          "elevenlabs-professional-1",
				Name:        "专业男声",
				Engine:      "elevenlabs",
				Language:    "zh-CN",
				Gender:      "male",
				Style:       "professional",
				PreviewURL:  "/static/preview/elevenlabs_professional_1.mp3",
				Description: "适合面试场景的沉稳专业男声",
			},
			{
				ID:          "elevenlabs-professional-2",
				Name:        "专业女声",
				Engine:      "elevenlabs",
				Language:    "zh-CN",
				Gender:      "female",
				Style:       "professional",
				PreviewURL:  "/static/preview/elevenlabs_professional_2.mp3",
				Description: "适合面试场景的自信专业女声",
			},
			{
				ID:          "minimax-interview-1",
				Name:        "面试专家",
				Engine:      "minimax",
				Language:    "zh-CN",
				Gender:      "male",
				Style:       "professional",
				PreviewURL:  "/static/preview/minimax_interview_1.mp3",
				Description: "MiniMax面试场景专用音色",
			},
			// 陪伴场景温柔音色
			{
				ID:          "aliyun-warm-1",
				Name:        "温柔女声",
				Engine:      "aliyun",
				Language:    "zh-CN",
				Gender:      "female",
				Style:       "warm",
				PreviewURL:  "/static/preview/aliyun_warm_1.mp3",
				Description: "温暖亲切的女声，适合陪伴场景",
			},
			{
				ID:          "xunfei-companion-1",
				Name:        "知心姐姐",
				Engine:      "xunfei",
				Language:    "zh-CN",
				Gender:      "female",
				Style:       "warm",
				PreviewURL:  "/static/preview/xunfei_companion_1.mp3",
				Description: "像知心姐姐一样的温暖声音",
			},
			{
				ID:          "elevenlabs-energetic-1",
				Name:        "活力青年",
				Engine:      "elevenlabs",
				Language:    "zh-CN",
				Gender:      "male",
				Style:       "energetic",
				PreviewURL:  "/static/preview/elevenlabs_energetic_1.mp3",
				Description: "充满活力的年轻男声",
			},
		},
	}
}

// Synthesize 将文本合成为语音
func (m *MockTTSProvider) Synthesize(ctx context.Context, req tts.SynthesizeRequest) (tts.SynthesizeResult, error) {
	// 按字数估算时长，约每秒5个字
	charCount := len([]rune(req.Text))
	duration := float64(charCount) / 5.0

	// 根据速度调整时长
	if req.Speed > 0 {
		duration = duration / req.Speed
	}

	// 确定格式
	format := req.Format
	if format == "" {
		format = "mp3"
	}

	return tts.SynthesizeResult{
		AudioURL:   "/static/mock/tts_output.mp3",
		Duration:   duration,
		Format:     format,
		SampleRate: 24000,
		CharCount:  charCount,
	}, nil
}

// ListVoices 列出指定引擎支持的音色列表
func (m *MockTTSProvider) ListVoices(ctx context.Context, engine string) ([]tts.Voice, error) {
	if engine == "" {
		return m.voices, nil
	}

	var filtered []tts.Voice
	for _, voice := range m.voices {
		if voice.Engine == engine {
			filtered = append(filtered, voice)
		}
	}
	return filtered, nil
}

// GetVoice 获取指定音色的详细信息
func (m *MockTTSProvider) GetVoice(ctx context.Context, voiceID string) (tts.Voice, error) {
	for _, voice := range m.voices {
		if voice.ID == voiceID {
			return voice, nil
		}
	}
	return tts.Voice{}, fmt.Errorf("voice not found: %s", voiceID)
}

// GetSupportedEngines 获取支持的引擎列表
func (m *MockTTSProvider) GetSupportedEngines() []string {
	return []string{"elevenlabs", "minimax", "aliyun", "xunfei"}
}
