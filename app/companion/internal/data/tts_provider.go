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
	defaultVolcengineBaseURL = "https://openspeech.bytedance.com/api/v1/tts"
	defaultVolcengineCluster = "volcano_tts"
	defaultVolcengineFormat  = "mp3"
	defaultVolcengineAppID   = "makejob_companion"
	ttsSuccessCode           = 3000
)

// volcengineProvider 火山引擎 TTS 供应商实现
type volcengineProvider struct {
	apiKey     string
	appID      string
	cluster    string
	baseURL    string
	voiceType  string
	encoding   string
	httpClient *http.Client
}

// NewVolcengineProvider 创建火山引擎 TTS 供应商
func NewVolcengineProvider(apiKey, voiceType string) biz.TTSProvider {
	return &volcengineProvider{
		apiKey:    apiKey,
		appID:     defaultVolcengineAppID,
		cluster:   defaultVolcengineCluster,
		baseURL:   defaultVolcengineBaseURL,
		voiceType: voiceType,
		encoding:  defaultVolcengineFormat,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (p *volcengineProvider) GetSupportedEngines() []string {
	return []string{"volcengine"}
}

func (p *volcengineProvider) Synthesize(ctx context.Context, req biz.TTSRequest) (*biz.TTSResult, error) {
	voice := req.VoiceID
	if voice == "" {
		voice = p.voiceType
	}

	payload := map[string]interface{}{
		"app": map[string]interface{}{
			"appid":   p.appID,
			"token":   p.apiKey,
			"cluster": p.cluster,
		},
		"user": map[string]interface{}{
			"uid": "companion_service",
		},
		"audio": map[string]interface{}{
			"voice_type":  voice,
			"encoding":    p.encoding,
			"speed_ratio": 1.0,
		},
		"request": map[string]interface{}{
			"text":      req.Text,
			"operation": "query",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal volcengine tts request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build volcengine tts request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer;"+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call volcengine tts api: %w", err)
	}
	defer resp.Body.Close()

	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
		Data string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode volcengine tts response: %w", err)
	}
	if response.Code != ttsSuccessCode {
		return nil, fmt.Errorf("volcengine tts failed: code=%d message=%s", response.Code, response.Msg)
	}

	audioData, err := base64.StdEncoding.DecodeString(response.Data)
	if err != nil {
		return nil, fmt.Errorf("decode volcengine tts audio: %w", err)
	}

	return &biz.TTSResult{
		AudioURL:  "data:audio/mp3;base64," + response.Data,
		AudioData: audioData,
		Format:    "mp3",
	}, nil
}

// minimaxProvider MiniMax TTS 供应商实现
type minimaxProvider struct {
	apiKey     string
	groupID    string
	model      string
	voiceID    string
	httpClient *http.Client
}

// NewMiniMaxProvider 创建 MiniMax TTS 供应商
func NewMiniMaxProvider(apiKey, groupID, voiceID string) biz.TTSProvider {
	return &minimaxProvider{
		apiKey:  apiKey,
		groupID: groupID,
		model:   "speech-2.8-turbo",
		voiceID: voiceID,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (p *minimaxProvider) GetSupportedEngines() []string {
	return []string{"minimax"}
}

func (p *minimaxProvider) Synthesize(ctx context.Context, req biz.TTSRequest) (*biz.TTSResult, error) {
	voice := req.VoiceID
	if voice == "" {
		voice = p.voiceID
	}

	payload := map[string]interface{}{
		"model": p.model,
		"text":  req.Text,
		"voice_setting": map[string]interface{}{
			"voice_id": voice,
			"speed":    1.0,
			"vol":      1.0,
			"pitch":    0,
		},
		"audio_setting": map[string]interface{}{
			"sample_rate": 32000,
			"format":      "mp3",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal minimax tts request: %w", err)
	}

	url := "https://api.minimax.io/v1/t2a_v2"
	if p.groupID != "" {
		url += "?GroupId=" + p.groupID
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build minimax tts request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call minimax tts api: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read minimax tts response: %w", err)
	}

	var response struct {
		Data struct {
			Audio string `json:"audio"`
		} `json:"data"`
		BaseResp struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(rawBody, &response); err != nil {
		return nil, fmt.Errorf("decode minimax tts response: %w", err)
	}
	if response.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("minimax tts failed: code=%d message=%s", response.BaseResp.StatusCode, response.BaseResp.StatusMsg)
	}

	audioHex := strings.TrimSpace(response.Data.Audio)
	if audioHex == "" {
		return nil, fmt.Errorf("minimax tts returned empty audio")
	}

	// hex 解码
	audioBytes := make([]byte, len(audioHex)/2)
	for i := 0; i < len(audioHex); i += 2 {
		var b byte
		fmt.Sscanf(audioHex[i:i+2], "%x", &b)
		audioBytes[i/2] = b
	}

	encoded := base64.StdEncoding.EncodeToString(audioBytes)
	return &biz.TTSResult{
		AudioURL:  "data:audio/mp3;base64," + encoded,
		AudioData: audioBytes,
		Format:    "mp3",
	}, nil
}

// mimoProvider Xiaomi MiMo TTS 供应商实现
type mimoProvider struct {
	apiKey     string
	model      string
	voice      string
	httpClient *http.Client
}

// NewMiMoProvider 创建 Xiaomi MiMo TTS 供应商
func NewMiMoProvider(apiKey, model, voice string) biz.TTSProvider {
	if model == "" {
		model = "mimo-v2-tts"
	}
	if voice == "" {
		voice = "mimo_default"
	}
	return &mimoProvider{
		apiKey: apiKey,
		model:  model,
		voice:  voice,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (p *mimoProvider) GetSupportedEngines() []string {
	return []string{"xiaomi_mimo"}
}

func (p *mimoProvider) Synthesize(ctx context.Context, req biz.TTSRequest) (*biz.TTSResult, error) {
	voice := req.VoiceID
	if voice == "" {
		voice = p.voice
	}

	payload := map[string]interface{}{
		"model": p.model,
		"messages": []map[string]interface{}{
			{"role": "assistant", "content": req.Text},
		},
		"audio": map[string]interface{}{
			"voice":  voice,
			"format": "wav",
		},
		"modalities": []string{"text", "audio"},
		"stream":     false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal mimo tts request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.xiaomimimo.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build mimo tts request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.apiKey)
	
	// 调试日志
	fmt.Printf("[TTS MiMo] Request URL: %s\n", "https://api.xiaomimimo.com/v1/chat/completions")
	fmt.Printf("[TTS MiMo] API Key: %s\n", p.apiKey)
	fmt.Printf("[TTS MiMo] Model: %s, Voice: %s\n", p.model, voice)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call mimo tts api: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read mimo tts response: %w", err)
	}

	// 调试日志：打印响应状态码和内容
	fmt.Printf("[TTS MiMo] Response Status: %d\n", resp.StatusCode)
	fmt.Printf("[TTS MiMo] Response Body: %s\n", string(rawBody))

	var response struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
		Choices []struct {
			Message struct {
				Audio struct {
					Data string `json:"data"`
				} `json:"audio"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rawBody, &response); err != nil {
		return nil, fmt.Errorf("decode mimo tts response: %w", err)
	}
	if response.Error != nil && response.Error.Message != "" {
		return nil, fmt.Errorf("mimo tts failed: %s", response.Error.Message)
	}
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("mimo tts returned empty choices")
	}

	audioData := strings.TrimSpace(response.Choices[0].Message.Audio.Data)
	if audioData == "" {
		return nil, fmt.Errorf("mimo tts returned empty audio")
	}

	return &biz.TTSResult{
		AudioURL:  "data:audio/wav;base64," + audioData,
		Format:    "wav",
	}, nil
}
