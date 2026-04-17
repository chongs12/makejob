package volcengine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconfig "makejob-backend/internal/config"
	"makejob-backend/internal/tts"
)

// TestProviderSynthesizeCallsVolcengine 验证真实 TTS Provider 能正确请求火山云接口。
func TestProviderSynthesizeCallsVolcengine(t *testing.T) {
	audioPayload := base64.StdEncoding.EncodeToString([]byte("fake-audio"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); !strings.Contains(got, "token-123") {
			t.Fatalf("expected authorization header to contain token, got %q", got)
		}

		var payload requestPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		if payload.App.AppID != "app-123" {
			t.Fatalf("expected appid app-123, got %q", payload.App.AppID)
		}
		if payload.Audio.Encoding != "wav" {
			t.Fatalf("expected encoding wav, got %q", payload.Audio.Encoding)
		}
		if payload.Audio.VoiceType != "voice-a" {
			t.Fatalf("expected voice voice-a, got %q", payload.Audio.VoiceType)
		}

		_ = json.NewEncoder(w).Encode(responsePayload{
			Code:    ttsSuccessCode,
			Message: "success",
			Data:    audioPayload,
			Addition: struct {
				Duration string `json:"duration"`
			}{
				Duration: "2500",
			},
		})
	}))
	defer server.Close()

	provider, err := NewProvider(appconfig.VolcengineConfig{
		TTS: appconfig.VolcTTSConfig{
			BaseURL:     server.URL,
			AppID:       "app-123",
			AccessToken: "token-123",
			Cluster:     "volcano_tts",
			VoiceType:   "voice-a",
			Encoding:    "wav",
			SampleRate:  24000,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	result, err := provider.Synthesize(context.Background(), tts.SynthesizeRequest{
		Text:    "你好，火山云",
		VoiceID: "voice-a",
	})
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	if !strings.HasPrefix(result.AudioURL, "data:audio/wav;base64,") {
		t.Fatalf("expected data url output, got %q", result.AudioURL)
	}
	if result.Duration != 2.5 {
		t.Fatalf("expected duration 2.5, got %v", result.Duration)
	}
}

// TestChooseEncodingFallsBackToConfig 验证输出编码会回退到配置默认值。
func TestChooseEncodingFallsBackToConfig(t *testing.T) {
	if got := chooseEncoding("", "wav"); got != "wav" {
		t.Fatalf("expected fallback encoding wav, got %q", got)
	}
	if got := chooseEncoding("ogg", "wav"); got != "ogg_opus" {
		t.Fatalf("expected ogg to normalize to ogg_opus, got %q", got)
	}
}
