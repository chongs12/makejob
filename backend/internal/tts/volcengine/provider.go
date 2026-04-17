package volcengine

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	appconfig "makejob-backend/internal/config"
	"makejob-backend/internal/tts"
)

const (
	defaultTTSBaseURL    = "https://openspeech.bytedance.com/api/v1/tts"
	defaultTTSCluster    = "volcano_tts"
	defaultTTSSampleRate = 24000
	defaultTTSEncoding   = "mp3"
	defaultVoiceType     = "BV001_streaming"
	ttsSuccessCode       = 3000
)

// Provider 封装火山云 TTS 实现。
type Provider struct {
	baseURL     string
	appID       string
	accessToken string
	cluster     string
	voiceType   string
	encoding    string
	sampleRate  int
	speedRatio  float64
	volumeRatio float64
	pitchRatio  float64
	httpClient  *http.Client
}

// responsePayload 描述火山云 TTS 返回结构。
type responsePayload struct {
	ReqID    string `json:"reqid"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Sequence int    `json:"sequence"`
	Data     string `json:"data"`
	Addition struct {
		Duration string `json:"duration"`
	} `json:"addition"`
}

// requestPayload 描述火山云 TTS 请求结构。
type requestPayload struct {
	App struct {
		AppID   string `json:"appid"`
		Token   string `json:"token"`
		Cluster string `json:"cluster"`
	} `json:"app"`
	User struct {
		UID string `json:"uid"`
	} `json:"user"`
	Audio struct {
		VoiceType   string  `json:"voice_type"`
		Encoding    string  `json:"encoding"`
		Rate        int     `json:"rate"`
		SpeedRatio  float64 `json:"speed_ratio"`
		VolumeRatio float64 `json:"volume_ratio"`
		PitchRatio  float64 `json:"pitch_ratio"`
		Language    string  `json:"language,omitempty"`
	} `json:"audio"`
	Request struct {
		ReqID     string `json:"reqid"`
		Text      string `json:"text"`
		TextType  string `json:"text_type"`
		Operation string `json:"operation"`
	} `json:"request"`
}

// NewProvider 根据火山云配置创建真实 TTS Provider。
func NewProvider(cfg appconfig.VolcengineConfig) (*Provider, error) {
	baseURL := strings.TrimSpace(cfg.TTS.BaseURL)
	if baseURL == "" {
		baseURL = defaultTTSBaseURL
	}
	appID := strings.TrimSpace(cfg.TTS.AppID)
	accessToken := strings.TrimSpace(cfg.TTS.AccessToken)
	if appID == "" || accessToken == "" {
		return nil, fmt.Errorf("volcengine tts config missing app_id or access_token")
	}

	cluster := strings.TrimSpace(cfg.TTS.Cluster)
	if cluster == "" {
		cluster = defaultTTSCluster
	}
	voiceType := strings.TrimSpace(cfg.TTS.VoiceType)
	if voiceType == "" {
		voiceType = defaultVoiceType
	}
	encoding := normalizeEncoding(cfg.TTS.Encoding)
	sampleRate := cfg.TTS.SampleRate
	if sampleRate <= 0 {
		sampleRate = defaultTTSSampleRate
	}

	return &Provider{
		baseURL:     baseURL,
		appID:       appID,
		accessToken: accessToken,
		cluster:     cluster,
		voiceType:   voiceType,
		encoding:    encoding,
		sampleRate:  sampleRate,
		speedRatio:  ratioFromPercent(cfg.TTS.SpeedRatio, 1),
		volumeRatio: ratioFromPercent(cfg.TTS.VolumeRatio, 1),
		pitchRatio:  ratioFromPercent(cfg.TTS.PitchRatio, 1),
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}, nil
}

// Synthesize 调用火山云接口完成文本转语音。
func (p *Provider) Synthesize(ctx context.Context, req tts.SynthesizeRequest) (tts.SynthesizeResult, error) {
	payload := p.buildRequestPayload(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("marshal volcengine tts request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("build volcengine tts request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer; "+p.accessToken)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("call volcengine tts api: %w", err)
	}
	defer resp.Body.Close()

	var response responsePayload
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("decode volcengine tts response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return tts.SynthesizeResult{}, fmt.Errorf("volcengine tts http status %d: %s", resp.StatusCode, strings.TrimSpace(response.Message))
	}
	if response.Code != ttsSuccessCode {
		return tts.SynthesizeResult{}, fmt.Errorf("volcengine tts failed: code=%d message=%s", response.Code, strings.TrimSpace(response.Message))
	}
	if strings.TrimSpace(response.Data) == "" {
		return tts.SynthesizeResult{}, fmt.Errorf("volcengine tts returned empty audio data")
	}
	if _, err := base64.StdEncoding.DecodeString(response.Data); err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("decode volcengine tts audio data: %w", err)
	}

	outputEncoding := chooseEncoding(req.Format, p.encoding)
	return tts.SynthesizeResult{
		AudioURL:   buildDataURL(outputEncoding, response.Data),
		Duration:   parseDurationSeconds(response.Addition.Duration),
		Format:     outputEncoding,
		SampleRate: chooseSampleRate(req, p.sampleRate),
		CharCount:  len([]rune(strings.TrimSpace(req.Text))),
	}, nil
}

// ListVoices 返回当前配置可用的默认音色列表。
func (p *Provider) ListVoices(ctx context.Context, engine string) ([]tts.Voice, error) {
	_ = ctx
	if strings.TrimSpace(engine) != "" && !strings.EqualFold(strings.TrimSpace(engine), "volcengine") {
		return []tts.Voice{}, nil
	}
	return []tts.Voice{
		p.defaultVoice(),
	}, nil
}

// GetVoice 返回当前配置下的默认音色信息。
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
	return []string{"volcengine"}
}

// buildRequestPayload 组装火山云 TTS 请求体。
func (p *Provider) buildRequestPayload(req tts.SynthesizeRequest) requestPayload {
	payload := requestPayload{}
	payload.App.AppID = p.appID
	payload.App.Token = p.accessToken
	payload.App.Cluster = p.cluster
	payload.User.UID = extractUserID(req.Extra)
	payload.Audio.VoiceType = chooseVoiceType(req, p.voiceType)
	payload.Audio.Encoding = chooseEncoding(req.Format, p.encoding)
	payload.Audio.Rate = chooseSampleRate(req, p.sampleRate)
	payload.Audio.SpeedRatio = choosePositiveFloat(req.Speed, p.speedRatio)
	payload.Audio.VolumeRatio = choosePositiveFloat(req.Volume, p.volumeRatio)
	payload.Audio.PitchRatio = choosePositiveFloat(req.Pitch, p.pitchRatio)
	payload.Audio.Language = normalizeLanguage(req.Extra["language"])
	payload.Request.ReqID = extractRequestID(req.Extra)
	payload.Request.Text = strings.TrimSpace(req.Text)
	payload.Request.TextType = "plain"
	payload.Request.Operation = "query"
	return payload
}

// defaultVoice 生成当前配置对应的默认音色说明。
func (p *Provider) defaultVoice() tts.Voice {
	return tts.Voice{
		ID:          p.voiceType,
		Name:        "火山云默认音色",
		Engine:      "volcengine",
		Language:    "zh-CN",
		Gender:      "unknown",
		Style:       "default",
		Description: "基于 config.yaml 配置的火山云音色。",
	}
}

// chooseEncoding 优先使用请求格式，否则回退到配置默认编码。
func chooseEncoding(format string, fallback string) string {
	normalized := normalizeEncoding(format)
	if strings.TrimSpace(format) == "" && strings.TrimSpace(fallback) != "" {
		return normalizeEncoding(fallback)
	}
	return normalized
}

// chooseVoiceType 选择本次合成使用的音色。
func chooseVoiceType(req tts.SynthesizeRequest, fallback string) string {
	if strings.TrimSpace(req.VoiceID) != "" {
		return strings.TrimSpace(req.VoiceID)
	}
	return fallback
}

// chooseSampleRate 选择本次合成的采样率。
func chooseSampleRate(req tts.SynthesizeRequest, fallback int) int {
	if req.Extra != nil {
		if value := strings.TrimSpace(req.Extra["sample_rate"]); value != "" {
			var sampleRate int
			if _, err := fmt.Sscanf(value, "%d", &sampleRate); err == nil && sampleRate > 0 {
				return sampleRate
			}
		}
	}
	return fallback
}

// choosePositiveFloat 优先使用请求值，否则回退到默认值。
func choosePositiveFloat(value float64, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}

// ratioFromPercent 将百分比配置转换为倍率。
func ratioFromPercent(value int, fallback float64) float64 {
	if value <= 0 {
		return fallback
	}
	return float64(value) / 100
}

// normalizeEncoding 将输出格式归一化为火山云支持的编码名。
func normalizeEncoding(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "wav", "pcm", "mp3", "ogg_opus":
		return strings.ToLower(strings.TrimSpace(format))
	case "ogg":
		return "ogg_opus"
	default:
		return defaultTTSEncoding
	}
}

// normalizeLanguage 将语言参数标准化。
func normalizeLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "zh-cn", "zh":
		return "cn"
	case "en-us", "en":
		return "en"
	default:
		return ""
	}
}

// extractUserID 提取用户标识，缺失时自动生成。
func extractUserID(extra map[string]string) string {
	if extra != nil {
		if userID := strings.TrimSpace(extra["user_id"]); userID != "" {
			return userID
		}
	}
	return uuid.NewString()
}

// extractRequestID 提取请求标识，缺失时自动生成。
func extractRequestID(extra map[string]string) string {
	if extra != nil {
		if requestID := strings.TrimSpace(extra["request_id"]); requestID != "" {
			return requestID
		}
	}
	return uuid.NewString()
}

// buildDataURL 将音频内容拼成可直接使用的 data URL。
func buildDataURL(format string, encoded string) string {
	mimeType := "audio/" + format
	if format == "pcm" {
		mimeType = "audio/L16"
	}
	return "data:" + mimeType + ";base64," + encoded
}

// parseDurationSeconds 将毫秒字符串转成秒。
func parseDurationSeconds(value string) float64 {
	var durationMS float64
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%f", &durationMS); err != nil {
		return 0
	}
	return durationMS / 1000
}
