package factory

import (
	"fmt"
	"strings"

	"makejob-backend/internal/config"
	"makejob-backend/internal/tts"
	"makejob-backend/internal/tts/mock"
	ttsvolc "makejob-backend/internal/tts/volcengine"
)

const (
	// ProviderTypeMock 表示使用本地 Mock TTS。
	ProviderTypeMock = "mock"
	// ProviderTypeVolcengine 表示使用火山云真实 TTS。
	ProviderTypeVolcengine = "volcengine"
)

// NewTTSProvider 使用全局配置创建 TTS Provider。
func NewTTSProvider(providerType string) tts.TTSProvider {
	provider, err := NewTTSProviderWithConfig(providerType, config.GetConfig())
	if err == nil {
		return provider
	}
	return mock.NewMockTTSProvider()
}

// NewTTSProviderWithConfig 根据显式配置创建 TTS Provider。
func NewTTSProviderWithConfig(providerType string, cfg *config.Config) (tts.TTSProvider, error) {
	switch normalizeProviderType(providerType, cfg) {
	case ProviderTypeVolcengine:
		if cfg == nil {
			return nil, fmt.Errorf("tts provider config is nil")
		}
		return ttsvolc.NewProvider(cfg.Volcengine)
	case ProviderTypeMock:
		return mock.NewMockTTSProvider(), nil
	default:
		return mock.NewMockTTSProvider(), nil
	}
}

// normalizeProviderType 归一化 TTS Provider 类型。
func normalizeProviderType(providerType string, cfg *config.Config) string {
	normalized := strings.ToLower(strings.TrimSpace(providerType))
	if normalized != "" {
		return normalized
	}
	if cfg != nil && cfg.Volcengine.TTS.Enabled {
		return ProviderTypeVolcengine
	}
	return ProviderTypeMock
}
