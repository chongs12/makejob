package xiaomimimo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"makejob-backend/internal/tts"
)

// TestProviderSynthesizeCallsXiaomiMIMO 验证 MiMo Provider 会按官方 OpenAI 风格协议发起 TTS 请求。
func TestProviderSynthesizeCallsXiaomiMIMO(t *testing.T) {
	audioPayload := base64.StdEncoding.EncodeToString([]byte("fake-mimo-audio"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("api-key"); got != "mimo-key" {
			t.Fatalf("expected api-key header, got %q", got)
		}

		var payload requestPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		if payload.Model != defaultModelV25 {
			t.Fatalf("expected model %q, got %q", defaultModelV25, payload.Model)
		}
		if len(payload.Messages) != 1 || payload.Messages[0].Role != "assistant" {
			t.Fatalf("expected assistant message payload, got %+v", payload.Messages)
		}
		if payload.Audio.Voice != "Mia" {
			t.Fatalf("expected voice Mia, got %q", payload.Audio.Voice)
		}
		if payload.Audio.Format != "mp3" {
			t.Fatalf("expected format mp3, got %q", payload.Audio.Format)
		}

		_ = json.NewEncoder(w).Encode(responsePayload{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
					Audio   struct {
						Data       string `json:"data"`
						ID         string `json:"id"`
						Transcript string `json:"transcript"`
					} `json:"audio"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Message: struct {
						Content string `json:"content"`
						Audio   struct {
							Data       string `json:"data"`
							ID         string `json:"id"`
							Transcript string `json:"transcript"`
						} `json:"audio"`
					}{
						Audio: struct {
							Data       string `json:"data"`
							ID         string `json:"id"`
							Transcript string `json:"transcript"`
						}{
							Data:       audioPayload,
							ID:         "audio-1",
							Transcript: "hello",
						},
					},
					FinishReason: "stop",
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewProvider(Config{
		APIKey:              "mimo-key",
		BaseURL:             server.URL,
		Model:               defaultModelV25,
		Voice:               "Mia",
		Format:              "wav",
		MaxCompletionTokens: 512,
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	result, err := provider.Synthesize(context.Background(), tts.SynthesizeRequest{
		Text:   "Hello MiMo",
		Format: "mp3",
	})
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	if !strings.HasPrefix(result.AudioURL, "data:audio/mp3;base64,") {
		t.Fatalf("expected data url output, got %q", result.AudioURL)
	}
	if result.CharCount != len([]rune("Hello MiMo")) {
		t.Fatalf("expected char count %d, got %d", len([]rune("Hello MiMo")), result.CharCount)
	}
}

// TestNewProviderRejectsUnsupportedVoice 验证 MiMo Provider 会拒绝模型不支持的音色。
func TestNewProviderRejectsUnsupportedVoice(t *testing.T) {
	_, err := NewProvider(Config{
		APIKey: "mimo-key",
		Model:  defaultModelV2,
		Voice:  "Mia",
	})
	if err == nil {
		t.Fatalf("expected unsupported voice error, got nil")
	}
}

// TestNormalizeFormat 验证 MiMo 输出格式会被归一化到当前支持的官方取值。
func TestNormalizeFormat(t *testing.T) {
	if got := NormalizeFormat("pcm"); got != "pcm16" {
		t.Fatalf("expected pcm to normalize to pcm16, got %q", got)
	}
	if got := NormalizeFormat("ogg"); got != defaultFormat {
		t.Fatalf("expected unsupported format to fallback to %q, got %q", defaultFormat, got)
	}
}
