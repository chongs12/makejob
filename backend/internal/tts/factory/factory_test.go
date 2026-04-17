package factory

import (
	"testing"

	"makejob-backend/internal/config"
	mocktts "makejob-backend/internal/tts/mock"
	realtts "makejob-backend/internal/tts/volcengine"
)

// TestNewTTSProviderWithConfigUsesVolcengine 验证启用火山云配置后优先返回真实 TTS。
func TestNewTTSProviderWithConfigUsesVolcengine(t *testing.T) {
	provider, err := NewTTSProviderWithConfig("", &config.Config{
		Volcengine: config.VolcengineConfig{
			TTS: config.VolcTTSConfig{
				Enabled:     true,
				AppID:       "app-1",
				AccessToken: "token-1",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewTTSProviderWithConfig returned error: %v", err)
	}
	if _, ok := provider.(*realtts.Provider); !ok {
		t.Fatalf("expected real tts provider, got %T", provider)
	}
}

// TestNewTTSProviderWithConfigFallsBackToMock 验证未启用真实配置时继续回退 Mock。
func TestNewTTSProviderWithConfigFallsBackToMock(t *testing.T) {
	provider, err := NewTTSProviderWithConfig("", &config.Config{})
	if err != nil {
		t.Fatalf("NewTTSProviderWithConfig returned error: %v", err)
	}
	if _, ok := provider.(*mocktts.MockTTSProvider); !ok {
		t.Fatalf("expected mock tts provider, got %T", provider)
	}
}
