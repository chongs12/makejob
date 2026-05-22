package service

import (
	"strings"
	"testing"

	"makejob-backend/internal/model"
)

// TestValidateTTSConfigInputAcceptsXiaomiMIMO 验证 MiMo 官方模型与音色组合可通过后端校验。
func TestValidateTTSConfigInputAcceptsXiaomiMIMO(t *testing.T) {
	authJSON, paramsJSON, err := ValidateTTSConfigInput(
		model.TTSEngineXiaomiMIMO,
		"Mia",
		`{"api_key":"mimo-key"}`,
		`{"model":"mimo-v2.5-tts","format":"pcm"}`,
	)
	if err != nil {
		t.Fatalf("ValidateTTSConfigInput returned error: %v", err)
	}
	if !strings.Contains(authJSON, `"api_key": "mimo-key"`) {
		t.Fatalf("expected normalized auth json to contain api key, got %s", authJSON)
	}
	if !strings.Contains(paramsJSON, `"model": "mimo-v2.5-tts"`) {
		t.Fatalf("expected normalized params json to contain normalized model, got %s", paramsJSON)
	}
	if !strings.Contains(paramsJSON, `"format": "pcm16"`) {
		t.Fatalf("expected normalized params json to contain normalized format, got %s", paramsJSON)
	}
}

// TestValidateTTSConfigInputRejectsUnsupportedXiaomiVoice 验证 MiMo 会拦截模型与音色不匹配的配置。
func TestValidateTTSConfigInputRejectsUnsupportedXiaomiVoice(t *testing.T) {
	_, _, err := ValidateTTSConfigInput(
		model.TTSEngineXiaomiMIMO,
		"Mia",
		`{"api_key":"mimo-key"}`,
		`{"model":"mimo-v2-tts"}`,
	)
	if err == nil {
		t.Fatalf("expected unsupported voice error, got nil")
	}
}
