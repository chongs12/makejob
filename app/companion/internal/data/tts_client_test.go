package data

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestTTSClientSynthesizeSuccess 验证 TTS 客户端会解码音频数据并保留兼容 data URI。
func TestTTSClientSynthesizeSuccess(t *testing.T) {
	const audioPayload = "fake-mp3-bytes"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":3000,"message":"ok","data":"` + base64.StdEncoding.EncodeToString([]byte(audioPayload)) + `"}`))
	}))
	defer server.Close()

	client := &ttsClient{
		apiKey:  "test-key",
		client:  server.Client(),
		baseURL: server.URL,
	}

	result, err := client.Synthesize(context.Background(), "你好", "test-voice")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(result.AudioData) != audioPayload {
		t.Fatalf("expected decoded audio payload %q, got %q", audioPayload, string(result.AudioData))
	}
	if result.AudioURL == "" {
		t.Fatal("expected compatibility audio URL to be populated")
	}
}

// TestTTSClientSynthesizeInvalidBase64 验证非法 base64 响应会返回解码错误。
func TestTTSClientSynthesizeInvalidBase64(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":3000,"message":"ok","data":"***invalid***"}`))
	}))
	defer server.Close()

	client := &ttsClient{
		apiKey:  "test-key",
		client:  server.Client(),
		baseURL: server.URL,
	}

	if _, err := client.Synthesize(context.Background(), "你好", "test-voice"); err == nil {
		t.Fatal("expected decode error, got nil")
	}
}
