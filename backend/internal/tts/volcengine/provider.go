package volcengine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	appconfig "makejob-backend/internal/config"
	"makejob-backend/internal/tts"
)

const (
	defaultLegacyTTSBaseURL = "https://openspeech.bytedance.com/api/v1/tts"
	defaultV3TTSBaseURL     = "https://openspeech.bytedance.com/api/v3/tts/unidirectional"
	defaultTTSCluster       = "volcano_tts"
	defaultTTSSampleRate    = 24000
	defaultTTSEncoding      = "mp3"
	defaultVoiceType        = "BV001_streaming"
	ttsSuccessCode          = 3000
	v3SuccessCode           = 20000000
)

// Provider 封装火山引擎 TTS 实现，并在配置 resource_id 后自动切换到 V3 接口。
type Provider struct {
	baseURL     string
	apiKey      string
	appID       string
	accessToken string
	cluster     string
	resourceID  string
	voiceType   string
	encoding    string
	sampleRate  int
	speedRatio  float64
	volumeRatio float64
	pitchRatio  float64
	useV3       bool
	httpClient  *http.Client
}

// legacyResponsePayload 描述火山旧版 V1 TTS 返回结构。
type legacyResponsePayload struct {
	ReqID    string `json:"reqid"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Sequence int    `json:"sequence"`
	Data     string `json:"data"`
	Addition struct {
		Duration string `json:"duration"`
	} `json:"addition"`
}

// legacyRequestPayload 描述火山旧版 V1 TTS 请求结构。
type legacyRequestPayload struct {
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

// v3RequestPayload 描述火山 V3 单向流式 TTS 请求体。
type v3RequestPayload struct {
	User struct {
		UID string `json:"uid"`
	} `json:"user"`
	ReqParams v3ReqParams `json:"req_params"`
}

// v3ReqParams 描述火山 V3 合成参数。
type v3ReqParams struct {
	Text        string        `json:"text"`
	Speaker     string        `json:"speaker"`
	AudioParams v3AudioParams `json:"audio_params"`
}

// v3AudioParams 描述火山 V3 输出音频参数。
type v3AudioParams struct {
	Format       string `json:"format"`
	SampleRate   int    `json:"sample_rate,omitempty"`
	SpeechRate   int    `json:"speech_rate,omitempty"`
	LoudnessRate int    `json:"loudness_rate,omitempty"`
	PitchRate    int    `json:"pitch_rate,omitempty"`
}

// v3ChunkPayload 描述火山 V3 流式返回中的单个 JSON 片段。
type v3ChunkPayload struct {
	Code      int    `json:"code"`
	Status    int    `json:"status"`
	Message   string `json:"message"`
	Data      string `json:"data"`
	Audio     string `json:"audio"`
	IsEnd     bool   `json:"is_end"`
	TraceID   string `json:"trace_id"`
	RequestID string `json:"request_id"`
	Addition  struct {
		Duration string `json:"duration"`
	} `json:"addition"`
}

// NewProvider 根据火山配置创建真实 TTS Provider。
func NewProvider(cfg appconfig.VolcengineConfig) (*Provider, error) {
	resourceID := strings.TrimSpace(cfg.TTS.ResourceID)
	useV3 := resourceID != ""
	baseURL := strings.TrimSpace(cfg.TTS.BaseURL)
	if baseURL == "" {
		if useV3 {
			baseURL = defaultV3TTSBaseURL
		} else {
			baseURL = defaultLegacyTTSBaseURL
		}
	}
	apiKey := strings.TrimSpace(cfg.TTS.APIKey)
	appID := strings.TrimSpace(cfg.TTS.AppID)
	accessToken := strings.TrimSpace(cfg.TTS.AccessToken)
	if apiKey == "" && (appID == "" || accessToken == "") {
		return nil, fmt.Errorf("volcengine tts config missing api_key or app_id/access_token")
	}
	if useV3 && resourceID == "" {
		return nil, fmt.Errorf("volcengine tts config missing resource_id")
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
		apiKey:      apiKey,
		appID:       appID,
		accessToken: accessToken,
		cluster:     cluster,
		resourceID:  resourceID,
		voiceType:   voiceType,
		encoding:    encoding,
		sampleRate:  sampleRate,
		speedRatio:  ratioFromPercent(cfg.TTS.SpeedRatio, 1),
		volumeRatio: ratioFromPercent(cfg.TTS.VolumeRatio, 1),
		pitchRatio:  ratioFromPercent(cfg.TTS.PitchRatio, 1),
		useV3:       useV3 || strings.Contains(strings.ToLower(baseURL), "/api/v3/"),
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}, nil
}

// Synthesize 根据当前配置自动选择火山 V1 或 V3 接口完成文本转语音。
func (p *Provider) Synthesize(ctx context.Context, req tts.SynthesizeRequest) (tts.SynthesizeResult, error) {
	if p.useV3 {
		return p.synthesizeV3(ctx, req)
	}
	return p.synthesizeLegacy(ctx, req)
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

// synthesizeLegacy 调用火山旧版 V1 TTS 接口完成文本转语音。
func (p *Provider) synthesizeLegacy(ctx context.Context, req tts.SynthesizeRequest) (tts.SynthesizeResult, error) {
	payload := p.buildLegacyRequestPayload(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("marshal volcengine legacy tts request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("build volcengine legacy tts request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer; "+p.accessToken)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("call volcengine legacy tts api: %w", err)
	}
	defer resp.Body.Close()

	var response legacyResponsePayload
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("decode volcengine legacy tts response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return tts.SynthesizeResult{}, fmt.Errorf("volcengine legacy tts http status %d: %s", resp.StatusCode, strings.TrimSpace(response.Message))
	}
	if response.Code != ttsSuccessCode {
		return tts.SynthesizeResult{}, fmt.Errorf("volcengine legacy tts failed: code=%d message=%s", response.Code, strings.TrimSpace(response.Message))
	}
	if strings.TrimSpace(response.Data) == "" {
		return tts.SynthesizeResult{}, fmt.Errorf("volcengine legacy tts returned empty audio data")
	}
	if _, err := base64.StdEncoding.DecodeString(response.Data); err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("decode volcengine legacy tts audio data: %w", err)
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

// synthesizeV3 调用火山 V3 单向流式接口，并将多段音频片段拼成完整音频。
func (p *Provider) synthesizeV3(ctx context.Context, req tts.SynthesizeRequest) (tts.SynthesizeResult, error) {
	payload := p.buildV3RequestPayload(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("marshal volcengine v3 tts request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("build volcengine v3 tts request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if strings.TrimSpace(p.apiKey) != "" {
		httpReq.Header.Set("X-Api-Key", p.apiKey)
	} else {
		httpReq.Header.Set("X-Api-App-Key", p.appID)
		httpReq.Header.Set("X-Api-Access-Key", p.accessToken)
	}
	httpReq.Header.Set("X-Api-Resource-Id", p.resourceID)
	httpReq.Header.Set("X-Api-Request-Id", extractRequestID(req.Extra))

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return tts.SynthesizeResult{}, fmt.Errorf("call volcengine v3 tts api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body)
		return tts.SynthesizeResult{}, fmt.Errorf("volcengine v3 tts http status %d: %s", resp.StatusCode, strings.TrimSpace(string(errorBody)))
	}

	audioBytes, duration, err := parseV3AudioStream(resp.Body)
	if err != nil {
		return tts.SynthesizeResult{}, err
	}
	if len(audioBytes) == 0 {
		return tts.SynthesizeResult{}, fmt.Errorf("volcengine v3 tts returned empty audio data")
	}

	outputEncoding := chooseEncoding(req.Format, p.encoding)
	return tts.SynthesizeResult{
		AudioURL:   buildBinaryDataURL(outputEncoding, audioBytes),
		Duration:   duration,
		Format:     outputEncoding,
		SampleRate: chooseSampleRate(req, p.sampleRate),
		CharCount:  len([]rune(strings.TrimSpace(req.Text))),
	}, nil
}

// buildLegacyRequestPayload 组装火山旧版 V1 TTS 请求体。
func (p *Provider) buildLegacyRequestPayload(req tts.SynthesizeRequest) legacyRequestPayload {
	payload := legacyRequestPayload{}
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

// buildV3RequestPayload 组装火山 V3 单向流式 TTS 请求体。
func (p *Provider) buildV3RequestPayload(req tts.SynthesizeRequest) v3RequestPayload {
	payload := v3RequestPayload{}
	payload.User.UID = extractUserID(req.Extra)
	payload.ReqParams.Text = strings.TrimSpace(req.Text)
	payload.ReqParams.Speaker = chooseVoiceType(req, p.voiceType)
	payload.ReqParams.AudioParams.Format = normalizeV3Encoding(chooseEncoding(req.Format, p.encoding))
	payload.ReqParams.AudioParams.SampleRate = chooseSampleRate(req, p.sampleRate)
	payload.ReqParams.AudioParams.SpeechRate = ratioToDelta(choosePositiveFloat(req.Speed, p.speedRatio), -50, 100)
	payload.ReqParams.AudioParams.LoudnessRate = ratioToDelta(choosePositiveFloat(req.Volume, p.volumeRatio), -100, 100)
	payload.ReqParams.AudioParams.PitchRate = ratioToDelta(choosePositiveFloat(req.Pitch, p.pitchRatio), -100, 100)
	return payload
}

// defaultVoice 生成当前配置对应的默认音色说明。
func (p *Provider) defaultVoice() tts.Voice {
	return tts.Voice{
		ID:          p.voiceType,
		Name:        "火山引擎默认音色",
		Engine:      "volcengine",
		Language:    "zh-CN",
		Gender:      "unknown",
		Style:       "default",
		Description: "基于 config.yaml 配置的火山引擎音色。",
	}
}

// parseV3AudioStream 解析火山 V3 单向流式返回，并拼接所有音频数据块。
func parseV3AudioStream(reader interface {
	Read(p []byte) (n int, err error)
}) ([]byte, float64, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	audioBuffer := bytes.Buffer{}
	var duration float64
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}

		var chunk v3ChunkPayload
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}

		if !isV3SuccessCode(chunk.Code) {
			return nil, 0, fmt.Errorf("volcengine v3 tts failed: code=%d message=%s", chunk.Code, strings.TrimSpace(chunk.Message))
		}
		audioChunk := firstNonEmpty(strings.TrimSpace(chunk.Data), strings.TrimSpace(chunk.Audio))
		if audioChunk != "" {
			decoded, err := base64.StdEncoding.DecodeString(audioChunk)
			if err != nil {
				return nil, 0, fmt.Errorf("decode volcengine v3 tts audio chunk: %w", err)
			}
			audioBuffer.Write(decoded)
		}
		if duration == 0 {
			duration = parseDurationSeconds(chunk.Addition.Duration)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("read volcengine v3 tts stream: %w", err)
	}
	return audioBuffer.Bytes(), duration, nil
}

// isV3SuccessCode 判断火山 V3 语音接口返回码是否表示成功。
func isV3SuccessCode(code int) bool {
	return code == 0 || code == ttsSuccessCode || code == v3SuccessCode
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

// ratioToDelta 将倍率转换为 V3 所需的线性偏移值，并限制到允许范围内。
func ratioToDelta(value float64, minValue int, maxValue int) int {
	delta := int((value - 1) * 100)
	if delta < minValue {
		return minValue
	}
	if delta > maxValue {
		return maxValue
	}
	return delta
}

// normalizeEncoding 将输出格式归一化为火山支持的编码名。
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

// normalizeV3Encoding 将编码名转换为火山 V3 接口使用的格式名。
func normalizeV3Encoding(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "ogg_opus":
		return "ogg"
	default:
		return strings.ToLower(strings.TrimSpace(format))
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

// buildDataURL 将 base64 音频内容拼成可直接使用的 data URL。
func buildDataURL(format string, encoded string) string {
	mimeType := "audio/" + format
	if format == "pcm" {
		mimeType = "audio/L16"
	}
	return "data:" + mimeType + ";base64," + encoded
}

// buildBinaryDataURL 将原始二进制音频内容编码成可直接使用的 data URL。
func buildBinaryDataURL(format string, rawBytes []byte) string {
	return buildDataURL(format, base64.StdEncoding.EncodeToString(rawBytes))
}

// parseDurationSeconds 将毫秒字符串转成秒。
func parseDurationSeconds(value string) float64 {
	var durationMS float64
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%f", &durationMS); err != nil {
		return 0
	}
	return durationMS / 1000
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
