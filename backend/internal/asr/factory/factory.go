package factory

import (
	"makejob-backend/internal/asr"
	"makejob-backend/internal/asr/mock"
)

// NewASRProvider 创建ASR Provider实例
// providerType: mock 或其他未来支持的类型
func NewASRProvider(providerType string) asr.ASRProvider {
	switch providerType {
	case "mock":
		return mock.NewMockASRProvider()
	default:
		// 默认返回Mock实现
		return mock.NewMockASRProvider()
	}
}
