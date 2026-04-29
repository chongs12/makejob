package runtime

import (
	"strings"
	"testing"

	"makejob-backend/internal/ai"
)

// TestValidateRuntimeConfigRejectsUnsupportedProvider 验证运行时会拒绝当前未接入的 Provider。
func TestValidateRuntimeConfigRejectsUnsupportedProvider(t *testing.T) {
	err := ValidateRuntimeConfig(map[string]string{
		ai.ConfigKeyProvider:       string(ai.ProviderTypeOpenAI),
		ai.ConfigKeyModel:          "gpt-4o-mini",
		ai.ConfigKeyAPIKey:         "test-key",
		ai.ConfigKeyTimeoutSeconds: "30",
	})
	if err == nil {
		t.Fatalf("expected unsupported provider validation error")
	}
	if !strings.Contains(err.Error(), "不支持主 Provider") {
		t.Fatalf("expected unsupported provider error, got %v", err)
	}
}

// TestValidateRuntimeConfigRejectsUnsupportedFallback 验证运行时会拒绝当前未启用的 fallback 配置。
func TestValidateRuntimeConfigRejectsUnsupportedFallback(t *testing.T) {
	err := ValidateRuntimeConfig(map[string]string{
		ai.ConfigKeyProvider:         string(ai.ProviderTypeEino),
		ai.ConfigKeyModel:            "gpt-4o-mini",
		ai.ConfigKeyAPIKey:           "test-key",
		ai.ConfigKeyFallbackProvider: string(ai.ProviderTypeEino),
		ai.ConfigKeyTimeoutSeconds:   "30",
	})
	if err == nil {
		t.Fatalf("expected unsupported fallback validation error")
	}
	if !strings.Contains(err.Error(), "不支持兜底 Provider") {
		t.Fatalf("expected unsupported fallback error, got %v", err)
	}
}

// TestValidateRuntimeConfigRejectsInvalidNumericRange 验证运行时会拒绝越界的数值配置。
func TestValidateRuntimeConfigRejectsInvalidNumericRange(t *testing.T) {
	err := ValidateRuntimeConfig(map[string]string{
		ai.ConfigKeyProvider:         string(ai.ProviderTypeEino),
		ai.ConfigKeyModel:            "gpt-4o-mini",
		ai.ConfigKeyAPIKey:           "test-key",
		ai.ConfigKeyTemperature:      "3",
		ai.ConfigKeyTopP:             "2",
		ai.ConfigKeyMaxTokens:        "0",
		ai.ConfigKeyTimeoutSeconds:   "-1",
		ai.ConfigKeyFallbackProvider: "",
	})
	if err == nil {
		t.Fatalf("expected invalid numeric range error")
	}
	message := err.Error()
	for _, part := range []string{ai.ConfigKeyTemperature, ai.ConfigKeyTopP, ai.ConfigKeyMaxTokens, ai.ConfigKeyTimeoutSeconds} {
		if !strings.Contains(message, part) {
			t.Fatalf("expected error to mention %s, got %v", part, err)
		}
	}
}

// TestValidateRuntimeConfigAcceptsCurrentSupportedEinoConfig 验证当前支持的 Eino 配置可以通过校验。
func TestValidateRuntimeConfigAcceptsCurrentSupportedEinoConfig(t *testing.T) {
	err := ValidateRuntimeConfig(map[string]string{
		ai.ConfigKeyProvider:       string(ai.ProviderTypeEino),
		ai.ConfigKeyModel:          "gpt-4o-mini",
		ai.ConfigKeyAPIKey:         "test-key",
		ai.ConfigKeyTemperature:    "0.7",
		ai.ConfigKeyTopP:           "0.9",
		ai.ConfigKeyMaxTokens:      "2048",
		ai.ConfigKeyTimeoutSeconds: "30",
		ai.ConfigKeyEnableStream:   "false",
	})
	if err != nil {
		t.Fatalf("expected current eino config to be valid, got %v", err)
	}
}
