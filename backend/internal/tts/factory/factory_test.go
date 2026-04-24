package factory

import (
	"testing"

	"makejob-backend/internal/config"
	minimaxtts "makejob-backend/internal/tts/minimax"
	realtts "makejob-backend/internal/tts/volcengine"
)

// TestNewTTSProviderWithConfigUsesMiniMax 验证启用 MiniMax 配置后可返回官方 TTS Provider。
func TestNewTTSProviderWithConfigUsesMiniMax(t *testing.T) {
	provider, err := NewTTSProviderWithConfig("", &config.Config{
		MiniMax: config.MiniMaxConfig{
			GroupID: "group-1",
			APIKey:  "minimax-key",
			TTS: config.MiniMaxTTSConfig{
				Enabled: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("NewTTSProviderWithConfig returned error: %v", err)
	}
	if _, ok := provider.(*minimaxtts.Provider); !ok {
		t.Fatalf("expected minimax tts provider, got %T", provider)
	}
}

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

// TestNewTTSProviderWithConfigReturnsErrorWithoutRealConfig 验证未启用真实配置时直接返回错误。
func TestNewTTSProviderWithConfigReturnsErrorWithoutRealConfig(t *testing.T) {
	provider, err := NewTTSProviderWithConfig("", &config.Config{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if provider != nil {
		t.Fatalf("expected nil provider, got %T", provider)
	}
}
