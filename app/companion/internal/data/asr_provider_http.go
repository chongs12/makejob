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
	defaultVolcASRHTTPURL = "https://openspeech.bytedance.com/api/v1/auc/recognize"
)

// volcengineHTTPASRProvider 火山引擎 ASR HTTP 批量识别实现（替代 WebSocket 流式方案）。
type volcengineHTTPASRProvider struct {
	appID       string
	accessToken string
	cluster     string
	baseURL     string
	language    string
	httpClient  *http.Client
}

// NewVolcengineHTTPASRProvider 创建火山引擎 HTTP ASR 供应商。
func NewVolcengineHTTPASRProvider(appID, accessToken, cluster, language string) biz.ASRProvider {
	if cluster == "" {
		cluster = "volcengine_streaming_common"
	}
	if language == "" {
		language = "zh-CN"
	}
	return &volcengineHTTPASRProvider{
		appID:       appID,
		accessToken: accessToken,
		cluster:     cluster,
		baseURL:     defaultVolcASRHTTPURL,
		language:    language,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *volcengineHTTPASRProvider) GetSupportedEngines() []string {
	return []string{"volcengine"}
}

// Recognize 调用火山引擎 HTTP ASR API 识别音频。
func (p *volcengineHTTPASRProvider) Recognize(ctx context.Context, req biz.ASRRequest) (*biz.ASRResult, error) {
	if len(req.AudioData) == 0 {
		return nil, fmt.Errorf("asr: empty audio data")
	}

	language := req.Language
	if strings.TrimSpace(language) == "" {
		language = p.language
	}

	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "pcm"
	}

	audioBase64 := base64.StdEncoding.EncodeToString(req.AudioData)

	payload := map[string]interface{}{
		"app": map[string]interface{}{
			"appid":   p.appID,
			"token":   p.accessToken,
			"cluster": p.cluster,
		},
		"user": map[string]interface{}{
			"uid": "companion_asr",
		},
		"audio": map[string]interface{}{
			"format":     format,
			"sample_rate": 16000,
			"language":   language,
			"data":       audioBase64,
		},
		"request": map[string]interface{}{
			"reqid":     fmt.Sprintf("asr-%d", time.Now().UnixMilli()),
			"sequence":  1,
			"nbest":     1,
			"result_type": "single",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal volcengine asr request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build volcengine asr request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer; "+p.accessToken)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call volcengine asr api: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read volcengine asr response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("volcengine asr returned status %d: %s", resp.StatusCode, string(rawBody))
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		ReqID   string `json:"reqid"`
		Result  []struct {
			Text       string  `json:"text"`
			Confidence float64 `json:"confidence"`
			Utterances []struct {
				Text string `json:"text"`
			} `json:"utterances"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, fmt.Errorf("decode volcengine asr response: %w", err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("volcengine asr failed: code=%d message=%s", result.Code, result.Message)
	}

	text := ""
	confidence := 0.0
	if len(result.Result) > 0 {
		text = strings.TrimSpace(result.Result[0].Text)
		confidence = result.Result[0].Confidence
		if text == "" && len(result.Result[0].Utterances) > 0 {
			text = strings.TrimSpace(result.Result[0].Utterances[0].Text)
		}
	}

	if text == "" {
		return nil, fmt.Errorf("volcengine asr returned empty text")
	}

	duration := float64(len(req.AudioData)) / float64(16000*2)
	if duration < 0.5 {
		duration = 0.5
	}

	return &biz.ASRResult{
		Text:       text,
		Confidence: confidence,
		Duration:   duration,
		Language:   language,
	}, nil
}
