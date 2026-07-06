package biz

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ASRProvider ASR 语音识别供应商接口（对称 TTSProvider）
type ASRProvider interface {
	// Recognize 识别音频数据，返回完整文本
	Recognize(ctx context.Context, req ASRRequest) (*ASRResult, error)
	// GetSupportedEngines 获取支持的引擎列表
	GetSupportedEngines() []string
}

// ASRRequest 语音识别请求
type ASRRequest struct {
	AudioData  []byte  // 音频数据
	Format     string  // pcm / wav / mp3
	SampleRate int     // 采样率
	Language   string  // zh-CN / en-US
	Engine     string  // 指定引擎（可选）
}

// ASRResult 语音识别结果
type ASRResult struct {
	Text       string  // 识别文本
	Confidence float64 // 置信度 0-1
	Duration   float64 // 音频时长（秒）
	Language   string  // 识别语言
}

// ASRConfig ASR 配置实体（对称 TTSConfig）
type ASRConfig struct {
	ID             uint           `gorm:"primaryKey;autoIncrement"`
	Name           string         `gorm:"size:100;not null;comment:配置名称"`
	Engine         string         `gorm:"size:32;not null;comment:ASR引擎(volcengine/whisper)"`
	AuthConfigJSON string         `gorm:"type:text;comment:鉴权配置JSON"`
	ParamsJSON     string         `gorm:"type:text;comment:额外参数JSON"`
	IsActive       bool           `gorm:"not null;default:true;comment:是否启用"`
	SortOrder      int            `gorm:"not null;default:0;comment:排序顺序"`
	CreatedAt      time.Time      `gorm:"not null;autoCreateTime"`
	UpdatedAt      time.Time      `gorm:"not null;autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (ASRConfig) TableName() string { return "asr_configs" }

// ASRClient ASR 语音识别客户端接口（简化版，对称 TTSClient）
type ASRClient interface {
	// Recognize 调用 ASR 服务识别语音，返回结构化结果
	Recognize(ctx context.Context, req ASRRequest) (*ASRResult, error)
}

// ASRConfigRepo ASR 配置仓库接口（对称 TTSConfigRepo）
type ASRConfigRepo interface {
	// GetByID 根据 ID 获取 ASR 配置
	GetByID(ctx context.Context, id uint) (*ASRConfig, error)
	// List 获取所有启用的 ASR 配置
	List(ctx context.Context) ([]ASRConfig, error)
	// Create 创建 ASR 配置
	Create(ctx context.Context, config *ASRConfig) error
	// Update 更新 ASR 配置
	Update(ctx context.Context, config *ASRConfig) error
	// Delete 删除 ASR 配置（软删除）
	Delete(ctx context.Context, id uint) error
}

// ASR 供应商引擎常量
const (
	ASREngineVolcengine  = "volcengine"
	ASREngineWhisper     = "whisper"
	ASREngineXiaomiMiMo  = "xiaomi_mimo"
)

// 场景默认 ASR 配置键（存储在 admin_configs 表中）
const (
	ASRDefaultConfigKeyCompanion = "asr_default_companion_config_id"
)

// ---------- Mock ASR Provider ----------

// MockASRProvider Mock ASR 实现，用于本地开发和测试。
type MockASRProvider struct{}

// NewMockASRProvider 创建 Mock ASR Provider。
func NewMockASRProvider() ASRProvider {
	return &MockASRProvider{}
}

// Recognize 模拟语音识别，返回固定文本。
func (m *MockASRProvider) Recognize(ctx context.Context, req ASRRequest) (*ASRResult, error) {
	if len(req.AudioData) == 0 {
		return nil, fmt.Errorf("asr: empty audio data")
	}

	// 模拟处理延迟
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 根据音频长度估算时长（假设 16kHz 16bit PCM）
	duration := float64(len(req.AudioData)) / float64(16000*2)
	if duration < 0.5 {
		duration = 0.5
	}

	return &ASRResult{
		Text:       "Go语言的goroutine是轻量级线程，由Go运行时管理",
		Confidence: 0.95,
		Duration:   duration,
		Language:   req.Language,
	}, nil
}

// GetSupportedEngines 返回 Mock 支持的引擎列表。
func (m *MockASRProvider) GetSupportedEngines() []string {
	return []string{"mock"}
}
