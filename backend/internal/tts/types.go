package tts

// Voice 音色信息
type Voice struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Engine      string `json:"engine"` // elevenlabs/minimax/aliyun/xunfei
	Language    string `json:"language"`
	Gender      string `json:"gender"` // male/female
	Style       string `json:"style"`  // professional/warm/energetic
	PreviewURL  string `json:"preview_url"`
	Description string `json:"description"`
}

// SynthesizeRequest 语音合成请求
type SynthesizeRequest struct {
	Text    string            `json:"text"`
	VoiceID string            `json:"voice_id"`
	Engine  string            `json:"engine"`
	Speed   float64           `json:"speed"`  // 0.5-2.0, default 1.0
	Pitch   float64           `json:"pitch"`  // 0.5-2.0, default 1.0
	Volume  float64           `json:"volume"` // 0.0-1.0, default 1.0
	Format  string            `json:"format"` // mp3/wav/ogg
	Extra   map[string]string `json:"extra"`  // 引擎特有参数
}

// SynthesizeResult 语音合成结果
type SynthesizeResult struct {
	AudioURL   string  `json:"audio_url"`
	Duration   float64 `json:"duration"` // 秒
	Format     string  `json:"format"`
	SampleRate int     `json:"sample_rate"`
	CharCount  int     `json:"char_count"`
}
