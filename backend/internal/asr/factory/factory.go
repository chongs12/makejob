package factory

import (
	"fmt"
	"strings"

	"makejob-backend/internal/asr"
	"makejob-backend/internal/asr/mock"
	asrvolc "makejob-backend/internal/asr/volcengine"
	"makejob-backend/internal/config"
)

const (
	// ProviderTypeMock 表示使用本地 Mock ASR。
	ProviderTypeMock = "mock"
	// ProviderTypeVolcengine 表示使用火山云真实 ASR。
	ProviderTypeVolcengine = "volcengine"
)

// NewASRProvider 使用全局配置创建 ASR Provider。
func NewASRProvider(providerType string) asr.ASRProvider {
	provider, err := NewASRProviderWithConfig(providerType, config.GetConfig())
	if err == nil {
		return provider
	}
	return mock.NewMockASRProvider()
}

// NewASRProviderWithConfig 根据显式配置创建 ASR Provider。
func NewASRProviderWithConfig(providerType string, cfg *config.Config) (asr.ASRProvider, error) {
	switch normalizeProviderType(providerType, cfg) {
	case ProviderTypeVolcengine:
		if cfg == nil {
			return nil, fmt.Errorf("asr provider config is nil")
		}
		return asrvolc.NewProvider(cfg.Volcengine)
	case ProviderTypeMock:
		return mock.NewMockASRProvider(), nil
	default:
		return mock.NewMockASRProvider(), nil
	}
}

// normalizeProviderType 归一化 ASR Provider 类型。
func normalizeProviderType(providerType string, cfg *config.Config) string {
	normalized := strings.ToLower(strings.TrimSpace(providerType))
	if normalized != "" {
		return normalized
	}
	if cfg != nil && cfg.Volcengine.ASR.Enabled {
		return ProviderTypeVolcengine
	}
	return ProviderTypeMock
}
