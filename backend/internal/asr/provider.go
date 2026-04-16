package asr

import "context"

// ASRProvider ASR语音识别服务接口
type ASRProvider interface {
	// Recognize 识别音频数据，返回完整文本
	Recognize(ctx context.Context, req RecognizeRequest) (RecognizeResult, error)
	// StartStream 开始流式识别会话
	StartStream(ctx context.Context, engine string, language string) (StreamSession, error)
	// GetSupportedEngines 获取支持的引擎列表
	GetSupportedEngines() []string
}
