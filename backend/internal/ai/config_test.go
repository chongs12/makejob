package ai

import "testing"

// TestDefaultRuntimeConfigPrefersEino 验证运行时默认配置改为真实服务优先。
func TestDefaultRuntimeConfigPrefersEino(t *testing.T) {
	config := DefaultRuntimeConfig()

	if got := config[ConfigKeyProvider]; got != string(ProviderTypeEino) {
		t.Fatalf("expected default provider to be %q, got %q", ProviderTypeEino, got)
	}
	if got := config[ConfigKeyFallbackProvider]; got != string(ProviderTypeMock) {
		t.Fatalf("expected default fallback provider to be %q, got %q", ProviderTypeMock, got)
	}
}

// TestNormalizeRuntimeConfigKeepsRealFirstDefaults 验证空配置归一化后仍保持真实优先策略。
func TestNormalizeRuntimeConfigKeepsRealFirstDefaults(t *testing.T) {
	config := NormalizeRuntimeConfig(map[string]string{})

	if got := config[ConfigKeyProvider]; got != string(ProviderTypeEino) {
		t.Fatalf("expected normalized provider to be %q, got %q", ProviderTypeEino, got)
	}
	if got := config[ConfigKeyFallbackProvider]; got != string(ProviderTypeMock) {
		t.Fatalf("expected normalized fallback provider to be %q, got %q", ProviderTypeMock, got)
	}
}
