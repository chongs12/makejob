package tts

import "context"

// TTSProvider TTS语音合成服务接口
type TTSProvider interface {
	// Synthesize 将文本合成为语音
	Synthesize(ctx context.Context, req SynthesizeRequest) (SynthesizeResult, error)
	// ListVoices 列出指定引擎支持的音色列表
	ListVoices(ctx context.Context, engine string) ([]Voice, error)
	// GetVoice 获取指定音色的详细信息
	GetVoice(ctx context.Context, voiceID string) (Voice, error)
	// GetSupportedEngines 获取支持的引擎列表
	GetSupportedEngines() []string
}
