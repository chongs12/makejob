package xiaomimimo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"makejob-backend/internal/tts"
)

const (
	defaultBaseURL             = "https://api.xiaomimimo.com/v1/chat/completions"
	defaultModelV2             = "mimo-v2-tts"
	defaultModelV25            = "mimo-v2.5-tts"
	defaultVoice               = "mimo_default"
	defaultFormat              = "wav"
	defaultTemperature         = 0.6
	defaultMaxCompletionTokens = 2048
	defaultTimeoutSeconds      = 45
)

var supportedVoicesByModel = map[string][]tts.Voice{
	defaultModelV2: {
		{ID: "mimo_default", Name: "mimo_default", Engine: "xiaomi_mimo", Language: "zh-CN", Gender: "unknown", Style: "default", Description: "MiMo V2 默认音色。"},
		{ID: "default_zh", Name: "default_zh", Engine: "xiaomi_mimo", Language: "zh-CN", Gender: "unknown", Style: "default", Description: "MiMo V2 中文音色。"},
		{ID: "default_en", Name: "default_en", Engine: "xiaomi_mimo", Language: "en-US", Gender: "unknown", Style: "default", Description: "MiMo V2 英文音色。"},
	},
	defaultModelV25: {
		{ID: "mimo_default", Name: "mimo_default", Engine: "xiaomi_mimo", Language: "zh-CN", Gender: "unknown", Style: "default", Description: "MiMo V2.5 默认音色。"},
		{ID: "冰糖", Name: "冰糖", Engine: "xiaomi_mimo", Language: "zh-CN", Gender: "female", Style: "warm", Description: "MiMo V2.5 中文预置音色。"},
		{ID: "茉莉", Name: "茉莉", Engine: "xiaomi_mimo", Language: "zh-CN", Gender: "female", Style: "soft", Description: "MiMo V2.5 中文预置音色。"},
		{ID: "苏打", Name: "苏打", Engine: "xiaomi_mimo", Language: "zh-CN", Gender: "female", Style: "bright", Description: "MiMo V2.5 中文预置音色。"},
		{ID: "白桦", Name: "白桦", Engine: "xiaomi_mimo", Language: "zh-CN", Gender: "male", Style: "steady", Description: "MiMo V2.5 中文预置音色。"},
		{ID: "Mia", Name: "Mia", Engine: "xiaomi_mimo", Language: "en-US", Gender: "female", Style: "natural", Description: "MiMo V2.5 英文预置音色。"},
		{ID: "Chloe", Name: "Chloe", Engine: "xiaomi_mimo", Language: "en-US", Gender: "female", Style: "natural", Description: "MiMo V2.5 英文预置音色。"},
		{ID: "Milo", Name: "Milo", Engine: "xiaomi_mimo", Language: "en-US", Gender: "male", Style: "natural", Description: "MiMo V2.5 英文预置音色。"},
		{ID: "Dean", Name: "Dean", Engine: "xiaomi_mimo", Language: "en-US", Gender: "male", Style: "natural", Description: "MiMo V2.5 英文预置音色。"},
	},
}

// Config 描述 Xiaomi MiMo TTS 的运行时配置。
type Config struct {
	APIKey              string
	BaseURL             string
	Model               string
	Voice               string
	Format              string
	Temperature         float64
	MaxCompletionTokens int
	TimeoutSeconds      int
}

// Provider 封装 Xiaomi MiMo 官方 OpenAI 风格 TTS 接口。
type Provider struct {
	baseURL             string
	apiKey              string
	model               string
	voice               string
	format              string
	temperature         float64
	maxCompletionTokens int
	httpClient          *http.Client
}

// requestPayload 描述 MiMo TTS 请求体。
type requestPayload struct {
	Model               string           `json:"model"`
	Messages            []requestMessage `json:"messages"`
	Audio               requestAudio     `json:"audio"`
	Modalities          []string         `json:"modalities"`
	Stream              bool             `json:"stream"`
	Temperature         float64          `json:"temperature,omitempty"`
	MaxCompletionTokens int              `json:"max_completion_tokens,omitempty"`
}

// requestMessage 描述 MiMo 对话消息。
type requestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// requestAudio 描述 MiMo 音频输出参数。
type requestAudio struct {
	Voice  string `json:"voice"`
	Format string `json:"format"`
}

// responsePayload 描述 MiMo OpenAI 风格返回结构。
type responsePayload struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
			Audio   struct {
				Data       string `json:"data"`
				ID         string `json:"id"`
				Transcript string `json:"transcript"`
			} `json:"audio"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		TotalTokens             int `json:"total_tokens"`
		CompletionTokensDetails struct {
			AudioTokens int `json:"audio_tokens"`
			TextTokens  int `json:"text_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
}

// NewProvider 根据 Xiaomi MiMo 配置创建真实 TTS Provider。
func NewProvider(cfg Config) (*Provider, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("xiaomi mimo tts config missing api_key")
	}

	model := NormalizeModel(cfg.Model)
	if !IsSupportedModel(model) {
		return nil, fmt.Errorf("unsupported xiaomi mimo model: %s", strings.TrimSpace(cfg.Model))
	}
	voice := firstNonEmpty(strings.TrimSpace(cfg.Voice), defaultVoice)
	if !IsSupportedVoice(model, voice) {
		return nil, fmt.Errorf("unsupported xiaomi mimo voice %q for model %q", voice, model)
	}

	timeoutSeconds := positiveIntOrDefault(cfg.TimeoutSeconds, defaultTimeoutSeconds)
	return &Provider{
		baseURL:             firstNonEmpty(strings.TrimSpace(cfg.BaseURL), defaultBaseURL),
		apiKey:              apiKey,
		model:               model,
		voice:               voice,
		format:              NormalizeFormat(cfg.Format),
		temperature:         positiveFloatOrDefault(cfg.Temperature, defaultTemperature),
		maxCompletionTokens: positiveIntOrDefault(cfg.MaxCompletionTokens, defaultMaxCompletionTokens),
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
	}, nil
}

// Synthesize 调用 Xiaomi MiMo 官方接口完成文本转语音。
func (p *Provider) Synthesize(ctx context.Context, req tts.SynthesizeRequest) (tts.SynthesizeResult, error) {
	payload := p.buildRequestPayload(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("marshal xiaomi mimo tts request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("build xiaomi mimo tts request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("call xiaomi mimo tts api: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("read xiaomi mimo tts response: %w", err)
	}

	var response responsePayload
	if err := json.Unmarshal(rawBody, &response); err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("decode xiaomi mimo tts response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return tts.SynthesizeResult{}, fmt.Errorf("xiaomi mimo tts http status %d: %s", resp.StatusCode, extractErrorMessage(response, rawBody))
	}
	if response.Error != nil && strings.TrimSpace(response.Error.Message) != "" {
		return tts.SynthesizeResult{}, fmt.Errorf("xiaomi mimo tts failed: %s", strings.TrimSpace(response.Error.Message))
	}

	audioData, err := extractAudioData(response)
	if err != nil {
		return tts.SynthesizeResult{}, err
	}
	if _, err := base64.StdEncoding.DecodeString(audioData); err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("decode xiaomi mimo tts audio data: %w", err)
	}

	format := NormalizeFormat(payload.Audio.Format)
	return tts.SynthesizeResult{
		AudioURL:   buildDataURL(format, audioData),
		Duration:   0,
		Format:     format,
		SampleRate: 0,
		CharCount:  len([]rune(strings.TrimSpace(req.Text))),
	}, nil
}

// ListVoices 返回当前 MiMo 模型支持的音色列表。
func (p *Provider) ListVoices(ctx context.Context, engine string) ([]tts.Voice, error) {
	_ = ctx
	if strings.TrimSpace(engine) != "" && !strings.EqualFold(strings.TrimSpace(engine), "xiaomi_mimo") {
		return []tts.Voice{}, nil
	}
	return append([]tts.Voice(nil), SupportedVoices(p.model)...), nil
}

// GetVoice 返回指定 MiMo 音色的详情。
func (p *Provider) GetVoice(ctx context.Context, voiceID string) (tts.Voice, error) {
	_ = ctx
	targetVoice := firstNonEmpty(strings.TrimSpace(voiceID), p.voice)
	for _, voice := range SupportedVoices(p.model) {
		if strings.EqualFold(strings.TrimSpace(voice.ID), targetVoice) {
			return voice, nil
		}
	}
	return tts.Voice{}, fmt.Errorf("voice not found: %s", voiceID)
}

// GetSupportedEngines 返回当前 Provider 支持的引擎标识。
func (p *Provider) GetSupportedEngines() []string {
	return []string{"xiaomi_mimo"}
}

// SupportedModels 返回当前实现允许的 MiMo 模型列表。
func SupportedModels() []string {
	return []string{defaultModelV2, defaultModelV25}
}

// SupportedVoices 返回指定模型允许的官方音色列表。
func SupportedVoices(model string) []tts.Voice {
	normalizedModel := NormalizeModel(model)
	voices, ok := supportedVoicesByModel[normalizedModel]
	if !ok {
		return []tts.Voice{}
	}
	return append([]tts.Voice(nil), voices...)
}

// IsSupportedModel 判断给定模型是否在当前支持范围内。
func IsSupportedModel(model string) bool {
	_, ok := supportedVoicesByModel[NormalizeModel(model)]
	return ok
}

// IsSupportedVoice 判断给定模型是否支持指定音色。
func IsSupportedVoice(model string, voice string) bool {
	normalizedVoice := strings.TrimSpace(voice)
	if normalizedVoice == "" {
		return false
	}
	for _, item := range SupportedVoices(model) {
		if strings.EqualFold(strings.TrimSpace(item.ID), normalizedVoice) {
			return true
		}
	}
	return false
}

// NormalizeModel 将模型名归一化到当前支持的官方取值。
func NormalizeModel(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	switch normalized {
	case defaultModelV2, defaultModelV25:
		return normalized
	default:
		return normalized
	}
}

// NormalizeFormat 将输出格式归一化到 MiMo 当前支持的取值。
func NormalizeFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "mp3", "wav", "pcm16":
		return strings.ToLower(strings.TrimSpace(format))
	case "pcm":
		return "pcm16"
	default:
		return defaultFormat
	}
}

// buildRequestPayload 组装 MiMo OpenAI 风格 TTS 请求体。
func (p *Provider) buildRequestPayload(req tts.SynthesizeRequest) requestPayload {
	return requestPayload{
		Model: p.model,
		Messages: []requestMessage{
			{
				Role:    "assistant",
				Content: strings.TrimSpace(req.Text),
			},
		},
		Audio: requestAudio{
			Voice:  chooseVoice(req.VoiceID, p.voice),
			Format: chooseFormat(req.Format, p.format),
		},
		Modalities:          []string{"text", "audio"},
		Stream:              false,
		Temperature:         p.temperature,
		MaxCompletionTokens: p.maxCompletionTokens,
	}
}

// extractAudioData 提取 MiMo 返回中的 base64 音频正文。
func extractAudioData(response responsePayload) (string, error) {
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("xiaomi mimo tts returned empty choices")
	}
	audioData := strings.TrimSpace(response.Choices[0].Message.Audio.Data)
	if audioData == "" {
		return "", fmt.Errorf("xiaomi mimo tts returned empty audio data")
	}
	return audioData, nil
}

// extractErrorMessage 优先返回结构化错误，缺失时回退为原始响应文本。
func extractErrorMessage(response responsePayload, rawBody []byte) string {
	if response.Error != nil && strings.TrimSpace(response.Error.Message) != "" {
		return strings.TrimSpace(response.Error.Message)
	}
	return strings.TrimSpace(string(rawBody))
}

// chooseVoice 优先使用请求传入的音色，否则回退到配置默认值。
func chooseVoice(voiceID string, fallback string) string {
	return firstNonEmpty(strings.TrimSpace(voiceID), fallback)
}

// chooseFormat 优先使用请求传入的格式，否则回退到配置默认值。
func chooseFormat(format string, fallback string) string {
	if strings.TrimSpace(format) == "" {
		return NormalizeFormat(fallback)
	}
	return NormalizeFormat(format)
}

// buildDataURL 将 base64 音频正文拼成浏览器可直接播放的 data URL。
func buildDataURL(format string, encoded string) string {
	mimeType := "audio/" + NormalizeFormat(format)
	if NormalizeFormat(format) == "pcm16" {
		mimeType = "audio/L16"
	}
	return "data:" + mimeType + ";base64," + encoded
}

// positiveIntOrDefault 在整型值无效时回退到默认值。
func positiveIntOrDefault(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// positiveFloatOrDefault 在浮点值无效时回退到默认值。
func positiveFloatOrDefault(value float64, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}

// firstNonEmpty 返回参数列表中的第一个非空字符串。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
