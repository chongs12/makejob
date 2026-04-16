package asr

// RecognizeRequest 语音识别请求
type RecognizeRequest struct {
	AudioData  []byte `json:"audio_data"`
	Format     string `json:"format"` // wav/mp3/pcm
	SampleRate int    `json:"sample_rate"`
	Language   string `json:"language"` // zh-CN/en-US
	Engine     string `json:"engine"`   // doubao/xunfei
}

// RecognizeResult 语音识别结果
type RecognizeResult struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"` // 0-1
	Duration   float64 `json:"duration"`
	Language   string  `json:"language"`
}

// StreamResult 流式识别结果
type StreamResult struct {
	Text       string  `json:"text"`
	IsFinal    bool    `json:"is_final"`
	Confidence float64 `json:"confidence"`
}

// StreamSession 流式识别会话接口
type StreamSession interface {
	// SendAudio 发送音频数据
	SendAudio(data []byte) error
	// ReceiveText 接收识别结果通道
	ReceiveText() <-chan StreamResult
	// Close 关闭会话
	Close() error
}
