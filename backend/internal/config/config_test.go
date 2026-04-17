package config

import "testing"

// TestAIRuntimeDefaultsUsesEinoAsPrimary 验证 AI 运行时默认配置改为 Eino 主用、Mock 兜底。
func TestAIRuntimeDefaultsUsesEinoAsPrimary(t *testing.T) {
	cfg := &Config{}

	runtimeConfig := cfg.AIRuntimeDefaults()
	if got := runtimeConfig["ai_provider"]; got != "eino" {
		t.Fatalf("expected ai_provider to be %q, got %q", "eino", got)
	}
	if got := runtimeConfig["ai_fallback_provider"]; got != "mock" {
		t.Fatalf("expected ai_fallback_provider to be %q, got %q", "mock", got)
	}
}

// TestAIRuntimeDefaultsPrefersExplicitProvider 验证显式配置仍能覆盖默认 provider。
func TestAIRuntimeDefaultsPrefersExplicitProvider(t *testing.T) {
	cfg := &Config{
		AI: AIConfig{
			Provider:         "mock",
			FallbackProvider: "eino",
		},
	}

	runtimeConfig := cfg.AIRuntimeDefaults()
	if got := runtimeConfig["ai_provider"]; got != "mock" {
		t.Fatalf("expected ai_provider to be %q, got %q", "mock", got)
	}
	if got := runtimeConfig["ai_fallback_provider"]; got != "eino" {
		t.Fatalf("expected ai_fallback_provider to be %q, got %q", "eino", got)
	}
}
