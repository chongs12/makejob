package bridge

import (
	"testing"

	"makejob-backend/internal/ai"
)

// TestNormalizeAIConfigsRejectsUnsupportedProvider 验证 bridge 会复用单体规则拒绝当前未接入的 Provider。
func TestNormalizeAIConfigsRejectsUnsupportedProvider(t *testing.T) {
	_, err := NormalizeAIConfigs(map[string]string{
		ai.ConfigKeyProvider:       string(ai.ProviderTypeOpenAI),
		ai.ConfigKeyModel:          "gpt-4o-mini",
		ai.ConfigKeyAPIKey:         "key",
		ai.ConfigKeyBaseURL:        "https://example.com",
		ai.ConfigKeyTimeoutSeconds: "30",
	})
	if err == nil {
		t.Fatal("expected unsupported provider to be rejected")
	}
}
