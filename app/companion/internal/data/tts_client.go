package data

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"makejob/app/companion/internal/biz"
	"makejob/app/companion/internal/conf"
)

// ttsClient 实现 biz.TTSClient 接口，调用火山引擎 TTS HTTP API
type ttsClient struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

// NewTTsClient 创建 TTS 客户端
func NewTTsClient(cfg *conf.TTS) biz.TTSClient {
	return &ttsClient{
		apiKey:  cfg.APIKey,
		client:  &http.Client{},
		baseURL: "https://openspeech.bytedance.com/api/v1/tts",
	}
}

// ttsRequest 火山引擎 TTS API 请求体
type ttsRequest struct {
	App struct {
		AppID   string `json:"appid"`
		Token   string `json:"token"`
		Cluster string `json:"cluster"`
	} `json:"app"`
	User struct {
		UID string `json:"uid"`
	} `json:"user"`
	Audio struct {
		VoiceType  string  `json:"voice_type"`
		Encoding   string  `json:"encoding"`
		SpeedRatio float64 `json:"speed_ratio"`
	} `json:"audio"`
	Request struct {
		ReqID     string `json:"reqid"`
		Text      string `json:"text"`
		Operation string `json:"operation"`
	} `json:"request"`
}

// ttsResponse 火山引擎 TTS API 响应体
type ttsResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data string `json:"data"` // base64 编码的音频数据
}

// Synthesize 调用火山引擎 TTS API 合成语音，返回结构化音频结果
func (c *ttsClient) Synthesize(ctx context.Context, text, voice string) (*biz.TTSAudio, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("TTS API key not configured")
	}

	reqBody := ttsRequest{}
	reqBody.App.AppID = "makejob_companion"
	reqBody.App.Token = c.apiKey
	reqBody.App.Cluster = "volcano_tts"
	reqBody.User.UID = "companion_service"
	reqBody.Audio.VoiceType = voice
	reqBody.Audio.Encoding = "mp3"
	reqBody.Audio.SpeedRatio = 1.0
	reqBody.Request.Text = text
	reqBody.Request.Operation = "query"

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal TTS request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create TTS request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer;"+c.apiKey)

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("TTS HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read TTS response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TTS API returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var ttsResp ttsResponse
	if err := json.Unmarshal(respBody, &ttsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal TTS response: %w", err)
	}

	if ttsResp.Code != 3000 {
		return nil, fmt.Errorf("TTS API error code %d: %s", ttsResp.Code, ttsResp.Msg)
	}

	if ttsResp.Data == "" {
		return nil, fmt.Errorf("TTS API returned empty audio data")
	}

	audioData, err := base64.StdEncoding.DecodeString(ttsResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode TTS audio data: %w", err)
	}

	// 以 audio_data 作为正式输出，audio_url 保留 data URI 仅用于兼容旧调用方。
	return &biz.TTSAudio{
		AudioData: audioData,
		AudioURL:  "data:audio/mp3;base64," + ttsResp.Data,
	}, nil
}
