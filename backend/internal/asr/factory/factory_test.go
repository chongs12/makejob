package factory

import (
	"testing"

	realasr "makejob-backend/internal/asr/volcengine"
	"makejob-backend/internal/config"
)

// TestNewASRProviderWithConfigUsesVolcengine 验证启用火山云配置后优先返回真实 ASR。
func TestNewASRProviderWithConfigUsesVolcengine(t *testing.T) {
	provider, err := NewASRProviderWithConfig("", &config.Config{
		Volcengine: config.VolcengineConfig{
			ASR: config.VolcASRConfig{
				Enabled:     true,
				AppID:       "app-1",
				AccessToken: "token-1",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewASRProviderWithConfig returned error: %v", err)
	}
	if _, ok := provider.(*realasr.Provider); !ok {
		t.Fatalf("expected real asr provider, got %T", provider)
	}
}

// TestNewASRProviderWithConfigReturnsErrorWithoutRealConfig 验证未启用真实配置时直接返回错误。
func TestNewASRProviderWithConfigReturnsErrorWithoutRealConfig(t *testing.T) {
	provider, err := NewASRProviderWithConfig("", &config.Config{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if provider != nil {
		t.Fatalf("expected nil provider, got %T", provider)
	}
}
