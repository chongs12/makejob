package minimax

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	appconfig "makejob-backend/internal/config"
	"makejob-backend/internal/tts"
	applogger "makejob-backend/pkg/logger"

	"go.uber.org/zap"
)

const (
	defaultTTSBaseURL        = "https://api.minimax.io/v1/t2a_v2"
	defaultTTSModel          = "speech-2.8-turbo"
	defaultTTSVoiceID        = "male-qn-jingying"
	defaultTTSEmotion        = "neutral"
	defaultTTSFormat         = "mp3"
	defaultTTSSampleRate     = 32000
	defaultTTSBitrate        = 128000
	defaultTTSChannel        = 1
	defaultTTSOutputFormat   = "hex"
	defaultTTSTimeoutSeconds = 45
	successStatusCode        = 0
)

// Provider 封装 MiniMax 官方 HTTP TTS 能力。
type Provider struct {
	baseURL        string
	groupID        string
	apiKey         string
	model          string
	voiceID        string
	emotion        string
	format         string
	sampleRate     int
	bitrate        int
	channel        int
	speed          float64
	volume         float64
	pitch          int
	subtitleEnable bool
	outputFormat   string
	httpClient     *http.Client
}

// requestPayload 描述 MiniMax TTS 请求体。
type requestPayload struct {
	Model          string              `json:"model"`
	Text           string              `json:"text"`
	Stream         bool                `json:"stream"`
	VoiceSetting   voiceSettingPayload `json:"voice_setting"`
	AudioSetting   audioSettingPayload `json:"audio_setting"`
	SubtitleEnable bool                `json:"subtitle_enable"`
	OutputFormat   string              `json:"output_format,omitempty"`
}

// voiceSettingPayload 描述音色和发声参数。
type voiceSettingPayload struct {
	VoiceID string  `json:"voice_id"`
	Speed   float64 `json:"speed"`
	Volume  float64 `json:"vol"`
	Pitch   int     `json:"pitch"`
	Emotion string  `json:"emotion,omitempty"`
}

// audioSettingPayload 描述输出音频格式参数。
type audioSettingPayload struct {
	SampleRate int    `json:"sample_rate"`
	Bitrate    int    `json:"bitrate"`
	Format     string `json:"format"`
	Channel    int    `json:"channel"`
}

// responsePayload 描述 MiniMax TTS 响应结构。
type responsePayload struct {
	Data struct {
		Audio     string `json:"audio"`
		AudioFile string `json:"audio_file"`
		Status    int    `json:"status"`
	} `json:"data"`
	ExtraInfo struct {
		AudioLength     int    `json:"audio_length"`
		AudioSampleRate int    `json:"audio_sample_rate"`
		AudioFormat     string `json:"audio_format"`
		UsageCharacters int    `json:"usage_characters"`
	} `json:"extra_info"`
	TraceID  string `json:"trace_id"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

// NewProvider 根据 MiniMax 配置创建官方 TTS Provider。
func NewProvider(cfg appconfig.MiniMaxConfig) (*Provider, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("minimax tts config missing api_key")
	}

	model := firstNonEmpty(strings.TrimSpace(cfg.TTS.Model), defaultTTSModel)
	voiceID := firstNonEmpty(strings.TrimSpace(cfg.TTS.VoiceID), defaultTTSVoiceID)
	format := normalizeFormat(cfg.TTS.Format)
	sampleRate := positiveIntOrDefault(cfg.TTS.SampleRate, defaultTTSSampleRate)
	bitrate := positiveIntOrDefault(cfg.TTS.Bitrate, defaultTTSBitrate)
	channel := positiveIntOrDefault(cfg.TTS.Channel, defaultTTSChannel)
	timeoutSeconds := positiveIntOrDefault(cfg.TTS.TimeoutSeconds, defaultTTSTimeoutSeconds)

	return &Provider{
		baseURL:        resolveBaseURL(cfg),
		groupID:        strings.TrimSpace(cfg.GroupID),
		apiKey:         apiKey,
		model:          model,
		voiceID:        voiceID,
		emotion:        firstNonEmpty(strings.TrimSpace(cfg.TTS.Emotion), defaultTTSEmotion),
		format:         format,
		sampleRate:     sampleRate,
		bitrate:        bitrate,
		channel:        channel,
		speed:          positiveFloatOrDefault(cfg.TTS.Speed, 1),
		volume:         positiveFloatOrDefault(cfg.TTS.Volume, 1),
		pitch:          cfg.TTS.Pitch,
		subtitleEnable: cfg.TTS.SubtitleEnable,
		outputFormat:   normalizeOutputFormat(cfg.TTS.OutputFormat),
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
	}, nil
}

// Synthesize 调用 MiniMax 官方接口完成文本转语音。
func (p *Provider) Synthesize(ctx context.Context, req tts.SynthesizeRequest) (tts.SynthesizeResult, error) {
	payload := p.buildRequestPayload(req)
	applogger.Info("minimax tts synthesize started",
		zap.String("url", p.baseURL),
		zap.String("group_id", p.groupID),
		zap.String("model", payload.Model),
		zap.String("voice_id", payload.VoiceSetting.VoiceID),
		zap.String("format", payload.AudioSetting.Format),
		zap.Int("text_length", len([]rune(payload.Text))),
	)
	body, err := json.Marshal(payload)
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("marshal minimax tts request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("build minimax tts request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("call minimax tts api: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("read minimax tts response: %w", err)
	}

	response, err := decodeResponsePayload(rawBody)
	if err != nil {
		applogger.Warn("minimax tts decode failed",
			zap.Int("http_status", resp.StatusCode),
			zap.String("content_type", strings.TrimSpace(resp.Header.Get("Content-Type"))),
			zap.String("raw_body", truncateForLog(string(rawBody), 2000)),
			zap.Error(err),
		)
		return tts.SynthesizeResult{}, fmt.Errorf("decode minimax tts response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return tts.SynthesizeResult{}, fmt.Errorf("minimax tts http status %d: %s", resp.StatusCode, strings.TrimSpace(response.BaseResp.StatusMsg))
	}
	if response.BaseResp.StatusCode != successStatusCode {
		return tts.SynthesizeResult{}, fmt.Errorf("minimax tts failed: code=%d message=%s", response.BaseResp.StatusCode, strings.TrimSpace(response.BaseResp.StatusMsg))
	}

	audioURL, format, err := p.resolveAudioURL(response, payload.AudioSetting.Format)
	if err != nil {
		return tts.SynthesizeResult{}, err
	}

	applogger.Info("minimax tts synthesize completed",
		zap.String("trace_id", strings.TrimSpace(response.TraceID)),
		zap.String("format", firstNonEmpty(strings.TrimSpace(response.ExtraInfo.AudioFormat), format)),
		zap.Int("sample_rate", positiveIntOrDefault(response.ExtraInfo.AudioSampleRate, payload.AudioSetting.SampleRate)),
		zap.Int("audio_length_ms", response.ExtraInfo.AudioLength),
	)

	return tts.SynthesizeResult{
		AudioURL:   audioURL,
		Duration:   float64(response.ExtraInfo.AudioLength) / 1000,
		Format:     firstNonEmpty(strings.TrimSpace(response.ExtraInfo.AudioFormat), format),
		SampleRate: positiveIntOrDefault(response.ExtraInfo.AudioSampleRate, payload.AudioSetting.SampleRate),
		CharCount:  positiveIntOrDefault(response.ExtraInfo.UsageCharacters, len([]rune(strings.TrimSpace(req.Text)))),
	}, nil
}

// ListVoices 返回当前已配置的默认 MiniMax 音色。
func (p *Provider) ListVoices(ctx context.Context, engine string) ([]tts.Voice, error) {
	_ = ctx
	if strings.TrimSpace(engine) != "" && !strings.EqualFold(strings.TrimSpace(engine), "minimax") {
		return []tts.Voice{}, nil
	}
	return []tts.Voice{p.defaultVoice()}, nil
}

// GetVoice 返回当前配置的默认 MiniMax 音色信息。
func (p *Provider) GetVoice(ctx context.Context, voiceID string) (tts.Voice, error) {
	_ = ctx
	voice := p.defaultVoice()
	if strings.TrimSpace(voiceID) == "" || strings.EqualFold(strings.TrimSpace(voiceID), voice.ID) {
		return voice, nil
	}
	return tts.Voice{}, fmt.Errorf("voice not found: %s", voiceID)
}

// GetSupportedEngines 返回当前 Provider 支持的引擎列表。
func (p *Provider) GetSupportedEngines() []string {
	return []string{"minimax"}
}

// buildRequestPayload 组装 MiniMax TTS 请求体。
func (p *Provider) buildRequestPayload(req tts.SynthesizeRequest) requestPayload {
	format := chooseFormat(req.Format, p.format)
	return requestPayload{
		Model:          chooseModel(req.Extra, p.model),
		Text:           strings.TrimSpace(req.Text),
		Stream:         false,
		SubtitleEnable: p.subtitleEnable,
		OutputFormat:   p.outputFormat,
		VoiceSetting: voiceSettingPayload{
			VoiceID: chooseVoiceID(req.VoiceID, p.voiceID),
			Speed:   positiveFloatOrDefault(req.Speed, p.speed),
			Volume:  positiveFloatOrDefault(req.Volume, p.volume),
			Pitch:   choosePitch(req, p.pitch),
			Emotion: chooseEmotion(req.Extra, p.emotion),
		},
		AudioSetting: audioSettingPayload{
			SampleRate: chooseSampleRate(req.Extra, p.sampleRate),
			Bitrate:    chooseBitrate(req.Extra, p.bitrate),
			Format:     format,
			Channel:    chooseChannel(req.Extra, p.channel),
		},
	}
}

// chooseModel 优先读取请求扩展参数中的模型名。
func chooseModel(extra map[string]string, fallback string) string {
	if extra == nil {
		return fallback
	}
	return firstNonEmpty(strings.TrimSpace(extra["model"]), fallback)
}

// resolveAudioURL 统一把 MiniMax 返回值转换为前端可直接播放的地址。
func (p *Provider) resolveAudioURL(response responsePayload, fallbackFormat string) (string, string, error) {
	format := firstNonEmpty(strings.TrimSpace(response.ExtraInfo.AudioFormat), fallbackFormat, p.format)
	if audioFile := strings.TrimSpace(response.Data.AudioFile); audioFile != "" {
		return audioFile, format, nil
	}

	audioHex := strings.TrimSpace(response.Data.Audio)
	if audioHex == "" {
		return "", format, fmt.Errorf("minimax tts returned empty audio data")
	}

	audioBytes, err := hex.DecodeString(audioHex)
	if err != nil {
		return "", format, fmt.Errorf("decode minimax tts audio hex: %w", err)
	}
	return buildDataURL(format, base64.StdEncoding.EncodeToString(audioBytes)), format, nil
}

// decodeResponsePayload 兼容解析 MiniMax 返回的标准 JSON 或被字符串包裹的 JSON。
func decodeResponsePayload(rawBody []byte) (responsePayload, error) {
	var response responsePayload
	trimmed := bytes.TrimSpace(rawBody)
	if len(trimmed) == 0 {
		return response, fmt.Errorf("empty response body")
	}

	if err := json.Unmarshal(trimmed, &response); err == nil {
		return response, nil
	}

	var wrapped string
	if err := json.Unmarshal(trimmed, &wrapped); err == nil {
		wrappedTrimmed := strings.TrimSpace(wrapped)
		if wrappedTrimmed == "" {
			return response, fmt.Errorf("empty wrapped response body")
		}
		if err := json.Unmarshal([]byte(wrappedTrimmed), &response); err == nil {
			return response, nil
		}
	}

	if _, err := strconv.ParseFloat(string(trimmed), 64); err == nil {
		return response, fmt.Errorf("unexpected numeric response body: %s", string(trimmed))
	}

	return response, fmt.Errorf("unsupported response body: %s", truncateForLog(string(trimmed), 400))
}

// defaultVoice 生成当前配置对应的默认音色说明。
func (p *Provider) defaultVoice() tts.Voice {
	return tts.Voice{
		ID:          p.voiceID,
		Name:        "MiniMax 默认音色",
		Engine:      "minimax",
		Language:    "zh-CN",
		Gender:      "unknown",
		Style:       p.emotion,
		Description: "基于 config.yaml 配置的 MiniMax 官方音色。",
	}
}

// resolveBaseURL 解析 MiniMax TTS 的最终请求地址，并兼容旧版 GroupId 查询参数。
func resolveBaseURL(cfg appconfig.MiniMaxConfig) string {
	groupID := strings.TrimSpace(cfg.GroupID)
	if baseURL := strings.TrimSpace(cfg.TTS.BaseURL); baseURL != "" {
		return appendGroupID(baseURL, groupID)
	}
	if baseURL := strings.TrimSpace(cfg.BaseURL); baseURL != "" {
		trimmed := strings.TrimRight(baseURL, "/")
		if strings.HasSuffix(trimmed, "/t2a_v2") {
			return appendGroupID(trimmed, groupID)
		}
		return appendGroupID(trimmed+"/t2a_v2", groupID)
	}
	return appendGroupID(defaultTTSBaseURL, groupID)
}

// appendGroupID 为旧版 MiniMax TTS 地址兼容追加 GroupId 查询参数。
func appendGroupID(rawURL string, groupID string) string {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return trimmedURL
	}

	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return trimmedURL
	}
	queryValues := parsedURL.Query()
	if strings.TrimSpace(groupID) != "" && strings.TrimSpace(queryValues.Get("GroupId")) == "" {
		queryValues.Set("GroupId", strings.TrimSpace(groupID))
	}
	parsedURL.RawQuery = queryValues.Encode()
	return parsedURL.String()
}

// chooseVoiceID 优先使用请求中的音色，否则回退到默认配置。
func chooseVoiceID(voiceID string, fallback string) string {
	return firstNonEmpty(strings.TrimSpace(voiceID), fallback)
}

// chooseEmotion 优先读取请求扩展参数中的情绪设置。
func chooseEmotion(extra map[string]string, fallback string) string {
	if extra == nil {
		return fallback
	}
	return firstNonEmpty(strings.TrimSpace(extra["emotion"]), fallback)
}

// chooseSampleRate 优先读取请求扩展参数中的采样率。
func chooseSampleRate(extra map[string]string, fallback int) int {
	return parsePositiveInt(extra, "sample_rate", fallback)
}

// chooseBitrate 优先读取请求扩展参数中的码率。
func chooseBitrate(extra map[string]string, fallback int) int {
	return parsePositiveInt(extra, "bitrate", fallback)
}

// chooseChannel 优先读取请求扩展参数中的声道数。
func chooseChannel(extra map[string]string, fallback int) int {
	return parsePositiveInt(extra, "channel", fallback)
}

// choosePitch 优先使用扩展参数中的整数 pitch 配置，否则回退到默认值。
func choosePitch(req tts.SynthesizeRequest, fallback int) int {
	if req.Extra == nil {
		return fallback
	}
	return parsePositiveOrNegativeInt(req.Extra["pitch"], fallback)
}

// parsePositiveInt 解析扩展参数中的正整数值。
func parsePositiveInt(extra map[string]string, key string, fallback int) int {
	if extra == nil {
		return fallback
	}
	value := strings.TrimSpace(extra[key])
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// parsePositiveOrNegativeInt 解析可正可负的整数配置。
func parsePositiveOrNegativeInt(value string, fallback int) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(trimmed, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

// chooseFormat 优先使用请求指定格式，否则回退到配置默认值。
func chooseFormat(format string, fallback string) string {
	normalized := normalizeFormat(format)
	if strings.TrimSpace(format) == "" {
		return normalizeFormat(fallback)
	}
	return normalized
}

// normalizeFormat 将音频格式归一化为 MiniMax 支持的名称。
func normalizeFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "mp3", "wav", "flac":
		return strings.ToLower(strings.TrimSpace(format))
	default:
		return defaultTTSFormat
	}
}

// normalizeOutputFormat 将输出格式限制到官方支持的 url 或 hex。
func normalizeOutputFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "url":
		return "url"
	default:
		return defaultTTSOutputFormat
	}
}

// buildDataURL 将音频内容拼成浏览器可直接播放的 data URL。
func buildDataURL(format string, encoded string) string {
	mimeType := "audio/" + normalizeFormat(format)
	return "data:" + mimeType + ";base64," + encoded
}

// truncateForLog 裁剪日志中的长文本，避免把完整音频或 HTML 全量打进日志。
func truncateForLog(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if limit <= 0 || len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit] + "...(truncated)"
}

// positiveIntOrDefault 在值非法时回退到默认值。
func positiveIntOrDefault(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// positiveFloatOrDefault 在值非法时回退到默认值。
func positiveFloatOrDefault(value float64, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
