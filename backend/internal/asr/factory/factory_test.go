package factory

import (
	"testing"

	"makejob-backend/internal/asr/mock"
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

// TestNewASRProviderWithConfigFallsBackToMock 验证未启用真实配置时继续回退 Mock。
func TestNewASRProviderWithConfigFallsBackToMock(t *testing.T) {
	provider, err := NewASRProviderWithConfig("", &config.Config{})
	if err != nil {
		t.Fatalf("NewASRProviderWithConfig returned error: %v", err)
	}
	if _, ok := provider.(*mock.MockASRProvider); !ok {
		t.Fatalf("expected mock asr provider, got %T", provider)
	}
}
