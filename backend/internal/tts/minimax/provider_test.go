package minimax

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconfig "makejob-backend/internal/config"
	"makejob-backend/internal/tts"
)

// TestProviderSynthesizeCallsMiniMax 验证 MiniMax Provider 会按官方协议发起 TTS 请求。
func TestProviderSynthesizeCallsMiniMax(t *testing.T) {
	audioPayload := hex.EncodeToString([]byte("fake-audio"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer minimax-key" {
			t.Fatalf("expected authorization header to contain api key, got %q", got)
		}

		var payload requestPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		if payload.Model != "speech-2.8-turbo" {
			t.Fatalf("expected model speech-2.8-turbo, got %q", payload.Model)
		}
		if payload.VoiceSetting.VoiceID != "male-qn-jingying" {
			t.Fatalf("expected configured voice id, got %q", payload.VoiceSetting.VoiceID)
		}
		if payload.AudioSetting.Format != "wav" {
			t.Fatalf("expected request format wav, got %q", payload.AudioSetting.Format)
		}

		_ = json.NewEncoder(w).Encode(responsePayload{
			Data: struct {
				Audio     string `json:"audio"`
				AudioFile string `json:"audio_file"`
				Status    int    `json:"status"`
			}{
				Audio: audioPayload,
			},
			ExtraInfo: struct {
				AudioLength     int    `json:"audio_length"`
				AudioSampleRate int    `json:"audio_sample_rate"`
				AudioFormat     string `json:"audio_format"`
				UsageCharacters int    `json:"usage_characters"`
			}{
				AudioLength:     1800,
				AudioSampleRate: 32000,
				AudioFormat:     "wav",
				UsageCharacters: 6,
			},
			BaseResp: struct {
				StatusCode int    `json:"status_code"`
				StatusMsg  string `json:"status_msg"`
			}{
				StatusCode: successStatusCode,
				StatusMsg:  "success",
			},
		})
	}))
	defer server.Close()

	provider, err := NewProvider(appconfig.MiniMaxConfig{
		GroupID: "group-123",
		APIKey:  "minimax-key",
		BaseURL: server.URL,
		TTS: appconfig.MiniMaxTTSConfig{
			Enabled:    true,
			Model:      "speech-2.8-turbo",
			VoiceID:    "male-qn-jingying",
			Format:     "mp3",
			SampleRate: 32000,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	result, err := provider.Synthesize(context.Background(), tts.SynthesizeRequest{
		Text:   "你好，MiniMax",
		Format: "wav",
	})
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	if !strings.HasPrefix(result.AudioURL, "data:audio/wav;base64,") {
		t.Fatalf("expected data url output, got %q", result.AudioURL)
	}
	if result.Duration != 1.8 {
		t.Fatalf("expected duration 1.8, got %v", result.Duration)
	}
}

// TestResolveBaseURLAppendsTTSPath 验证通用 BaseURL 会自动拼接 MiniMax TTS 路径。
func TestResolveBaseURLAppendsTTSPath(t *testing.T) {
	got := resolveBaseURL(appconfig.MiniMaxConfig{
		GroupID: "group-123",
		BaseURL: "https://api.minimaxi.com/v1",
	})
	if got != "https://api.minimaxi.com/v1/t2a_v2?GroupId=group-123" {
		t.Fatalf("expected minimax tts url, got %q", got)
	}
}
