package factory

import (
	"makejob-backend/internal/tts"
	"makejob-backend/internal/tts/mock"
)

// NewTTSProvider 创建TTS Provider实例
// providerType: mock 或其他未来支持的类型
func NewTTSProvider(providerType string) tts.TTSProvider {
	switch providerType {
	case "mock":
		return mock.NewMockTTSProvider()
	default:
		// 默认返回Mock实现
		return mock.NewMockTTSProvider()
	}
}
