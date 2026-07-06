package data

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

	"makejob/app/companion/internal/biz"
)

const (
	defaultMiMoASRBaseURL = "https://api.xiaomimimo.com/v1"
	defaultMiMoASRModel   = "mimo-v2.5-asr"
)

// mimoASRProvider 小米 MiMo ASR 供应商实现（OpenAI 兼容接口）。
type mimoASRProvider struct {
	apiKey     string
	baseURL    string
	model      string
	language   string
	httpClient *http.Client
}

// NewMiMoASRProvider 创建小米 MiMo ASR 供应商。
func NewMiMoASRProvider(apiKey, model, language, baseURL string) biz.ASRProvider {
	if model == "" {
		model = defaultMiMoASRModel
	}
	if language == "" {
		language = "auto"
	}
	if baseURL == "" {
		baseURL = defaultMiMoASRBaseURL
	}
	return &mimoASRProvider{
		apiKey:   apiKey,
		baseURL:  strings.TrimRight(baseURL, "/"),
		model:    model,
		language: language,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *mimoASRProvider) GetSupportedEngines() []string {
	return []string{"xiaomi_mimo"}
}

// normalizeMiMoASRLanguage 将 zh-CN/en-US 等格式转为 MiMo 接受的 zh/en/auto。
func normalizeMiMoASRLanguage(lang string) string {
	lower := strings.ToLower(strings.TrimSpace(lang))
	switch {
	case strings.HasPrefix(lower, "zh"):
		return "zh"
	case strings.HasPrefix(lower, "en"):
		return "en"
	default:
		return "auto"
	}
}

// Recognize 调用 MiMo ASR API 识别音频。
func (p *mimoASRProvider) Recognize(ctx context.Context, req biz.ASRRequest) (*biz.ASRResult, error) {
	if len(req.AudioData) == 0 {
		return nil, fmt.Errorf("asr: empty audio data")
	}

	language := req.Language
	if strings.TrimSpace(language) == "" {
		language = p.language
	}
	// MiMo ASR 只接受 zh / en / auto，不接受 zh-CN 格式
	language = normalizeMiMoASRLanguage(language)

	// 确定音频格式和 MIME 类型
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "wav"
	}
	mimeType := "audio/wav"
	if format == "mp3" {
		mimeType = "audio/mpeg"
	}

	// 构造 base64 data URL
	audioBase64 := base64.StdEncoding.EncodeToString(req.AudioData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, audioBase64)

	// 构造请求体（OpenAI 兼容格式）
	payload := map[string]interface{}{
		"model": p.model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "input_audio",
						"input_audio": map[string]interface{}{
							"data": dataURL,
						},
					},
				},
			},
		},
		"asr_options": map[string]interface{}{
			"language": language,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal mimo asr request: %w", err)
	}

	url := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build mimo asr request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call mimo asr api: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read mimo asr response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mimo asr returned status %d: %s", resp.StatusCode, string(rawBody))
	}

	// 解析 OpenAI 兼容响应
	var result struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, fmt.Errorf("decode mimo asr response: %w", err)
	}
	if result.Error != nil && result.Error.Message != "" {
		return nil, fmt.Errorf("mimo asr failed: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("mimo asr returned empty choices")
	}

	text := strings.TrimSpace(result.Choices[0].Message.Content)

	// 估算音频时长
	duration := float64(len(req.AudioData)) / float64(16000*2)
	if duration < 0.5 {
		duration = 0.5
	}

	return &biz.ASRResult{
		Text:       text,
		Confidence: 0.9, // MiMo 不返回置信度，给默认值
		Duration:   duration,
		Language:   language,
	}, nil
}
