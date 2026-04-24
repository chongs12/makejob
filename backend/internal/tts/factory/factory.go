package factory

import (
	"fmt"
	"strings"

	"makejob-backend/internal/config"
	"makejob-backend/internal/tts"
	ttsminimax "makejob-backend/internal/tts/minimax"
	ttsvolc "makejob-backend/internal/tts/volcengine"
)

const (
	// ProviderTypeMock 表示使用本地 Mock TTS。
	ProviderTypeMock = "mock"
	// ProviderTypeMiniMax 表示使用 MiniMax 官方 TTS。
	ProviderTypeMiniMax = "minimax"
	// ProviderTypeVolcengine 表示使用火山云真实 TTS。
	ProviderTypeVolcengine = "volcengine"
)

// NewTTSProvider 使用全局配置创建 TTS Provider。
func NewTTSProvider(providerType string) (tts.TTSProvider, error) {
	return NewTTSProviderWithConfig(providerType, config.GetConfig())
}

// NewTTSProviderWithConfig 根据显式配置创建 TTS Provider。
func NewTTSProviderWithConfig(providerType string, cfg *config.Config) (tts.TTSProvider, error) {
	switch normalizeProviderType(providerType, cfg) {
	case ProviderTypeMiniMax:
		if cfg == nil {
			return nil, fmt.Errorf("tts provider config is nil")
		}
		return ttsminimax.NewProvider(buildMiniMaxConfig(cfg))
	case ProviderTypeVolcengine:
		if cfg == nil {
			return nil, fmt.Errorf("tts provider config is nil")
		}
		return ttsvolc.NewProvider(cfg.Volcengine)
	case ProviderTypeMock:
		return nil, fmt.Errorf("tts provider mock is disabled")
	default:
		return nil, fmt.Errorf("tts provider is not configured")
	}
}

// normalizeProviderType 归一化 TTS Provider 类型。
func normalizeProviderType(providerType string, cfg *config.Config) string {
	normalized := strings.ToLower(strings.TrimSpace(providerType))
	if normalized != "" {
		return normalized
	}
	if cfg != nil && cfg.MiniMax.TTS.Enabled {
		return ProviderTypeMiniMax
	}
	if cfg != nil && cfg.Volcengine.TTS.Enabled {
		return ProviderTypeVolcengine
	}
	return ProviderTypeMock
}

// buildMiniMaxConfig 组装带有 API Key 回退逻辑的 MiniMax 配置。
func buildMiniMaxConfig(cfg *config.Config) config.MiniMaxConfig {
	if cfg == nil {
		return config.MiniMaxConfig{}
	}

	minimaxCfg := cfg.MiniMax
	if strings.TrimSpace(minimaxCfg.APIKey) == "" {
		minimaxCfg.APIKey = firstNonEmpty(cfg.AI.APIKey, cfg.Volcengine.Ark.APIKey)
	}
	if strings.TrimSpace(minimaxCfg.BaseURL) == "" {
		minimaxCfg.BaseURL = firstNonEmpty(cfg.AI.BaseURL, cfg.Volcengine.Ark.BaseURL)
	}
	return minimaxCfg
}

// firstNonEmpty 返回第一个非空字符串，用于配置回退。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
