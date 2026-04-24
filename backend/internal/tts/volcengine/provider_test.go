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

// TestProviderSynthesizeCallsVolcengineLegacy 验证未配置 resource_id 时仍走旧版 V1 接口。
func TestProviderSynthesizeCallsVolcengineLegacy(t *testing.T) {
	audioPayload := base64.StdEncoding.EncodeToString([]byte("fake-audio"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); !strings.Contains(got, "token-123") {
			t.Fatalf("expected authorization header to contain token, got %q", got)
		}

		var payload legacyRequestPayload
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

		_ = json.NewEncoder(w).Encode(legacyResponsePayload{
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

// TestProviderSynthesizeCallsVolcengineV3 验证配置 resource_id 时会走火山 V3 单向流式接口。
func TestProviderSynthesizeCallsVolcengineV3(t *testing.T) {
	audioPayload := base64.StdEncoding.EncodeToString([]byte("fake-v3-audio"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-App-Key") != "app-v3" {
			t.Fatalf("expected X-Api-App-Key header, got %q", r.Header.Get("X-Api-App-Key"))
		}
		if r.Header.Get("X-Api-Access-Key") != "token-v3" {
			t.Fatalf("expected X-Api-Access-Key header, got %q", r.Header.Get("X-Api-Access-Key"))
		}
		if r.Header.Get("X-Api-Resource-Id") != "seed-tts-2.0" {
			t.Fatalf("expected X-Api-Resource-Id header, got %q", r.Header.Get("X-Api-Resource-Id"))
		}

		var payload v3RequestPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		if payload.ReqParams.Speaker != "zh_female_vv_uranus_bigtts" {
			t.Fatalf("expected v3 speaker, got %q", payload.ReqParams.Speaker)
		}
		if payload.ReqParams.AudioParams.Format != "mp3" {
			t.Fatalf("expected v3 format mp3, got %q", payload.ReqParams.AudioParams.Format)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":20000000,"message":"OK","data":"` + audioPayload + `","addition":{"duration":"1200"}}` + "\n"))
	}))
	defer server.Close()

	provider, err := NewProvider(appconfig.VolcengineConfig{
		TTS: appconfig.VolcTTSConfig{
			BaseURL:     server.URL,
			AppID:       "app-v3",
			AccessToken: "token-v3",
			ResourceID:  "seed-tts-2.0",
			VoiceType:   "zh_female_vv_uranus_bigtts",
			Encoding:    "mp3",
			SampleRate:  24000,
			SpeedRatio:  100,
			VolumeRatio: 100,
			PitchRatio:  100,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	result, err := provider.Synthesize(context.Background(), tts.SynthesizeRequest{
		Text: "你好，V3 火山语音",
	})
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	if !strings.HasPrefix(result.AudioURL, "data:audio/mp3;base64,") {
		t.Fatalf("expected data url output, got %q", result.AudioURL)
	}
	if result.Duration != 1.2 {
		t.Fatalf("expected duration 1.2, got %v", result.Duration)
	}
}

// TestIsV3SuccessCode 验证火山 V3 返回的成功码不会再被误判为失败。
func TestIsV3SuccessCode(t *testing.T) {
	if !isV3SuccessCode(v3SuccessCode) {
		t.Fatalf("expected v3 success code %d to be accepted", v3SuccessCode)
	}
	if isV3SuccessCode(401) {
		t.Fatalf("expected non-success code to be rejected")
	}
}

// TestProviderSynthesizeCallsVolcengineV3WithAPIKey 验证配置 api_key 时会走新版单 Key 鉴权头。
func TestProviderSynthesizeCallsVolcengineV3WithAPIKey(t *testing.T) {
	audioPayload := base64.StdEncoding.EncodeToString([]byte("fake-v3-audio"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "new-api-key" {
			t.Fatalf("expected X-Api-Key header, got %q", r.Header.Get("X-Api-Key"))
		}
		if r.Header.Get("X-Api-App-Key") != "" {
			t.Fatalf("expected empty X-Api-App-Key header, got %q", r.Header.Get("X-Api-App-Key"))
		}
		if r.Header.Get("X-Api-Access-Key") != "" {
			t.Fatalf("expected empty X-Api-Access-Key header, got %q", r.Header.Get("X-Api-Access-Key"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":"` + audioPayload + `","addition":{"duration":"800"}}` + "\n"))
	}))
	defer server.Close()

	provider, err := NewProvider(appconfig.VolcengineConfig{
		TTS: appconfig.VolcTTSConfig{
			BaseURL:    server.URL,
			APIKey:     "new-api-key",
			ResourceID: "seed-tts-2.0",
			VoiceType:  "zh_female_vv_uranus_bigtts",
			Encoding:   "mp3",
			SampleRate: 24000,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	result, err := provider.Synthesize(context.Background(), tts.SynthesizeRequest{
		Text: "你好，V3 火山语音",
	})
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	if !strings.HasPrefix(result.AudioURL, "data:audio/mp3;base64,") {
		t.Fatalf("expected data url output, got %q", result.AudioURL)
	}
	if result.Duration != 0.8 {
		t.Fatalf("expected duration 0.8, got %v", result.Duration)
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
