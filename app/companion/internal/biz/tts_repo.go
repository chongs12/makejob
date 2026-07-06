package biz

import "context"

// TTSConfigRepo TTS 配置仓库接口
type TTSConfigRepo interface {
	// GetByID 根据 ID 获取 TTS 配置
	GetByID(ctx context.Context, id uint) (*TTSConfig, error)
	// List 获取所有启用的 TTS 配置
	List(ctx context.Context) ([]TTSConfig, error)
}

// Live2DModelRepo Live2D 模型仓库接口
type Live2DModelRepo interface {
	// GetByID 根据 ID 获取 Live2D 模型
	GetByID(ctx context.Context, id uint) (*Live2DModel, error)
	// GetByKey 根据 key 获取 Live2D 模型（支持 "db:123" 格式）
	GetByKey(ctx context.Context, key string) (*Live2DModel, error)
}

// AdminConfigRepo 管理后台配置仓库接口
type AdminConfigRepo interface {
	// GetByKey 根据配置键获取配置值
	GetByKey(ctx context.Context, key string) (*AdminConfig, error)
	// Upsert 创建或更新配置
	Upsert(ctx context.Context, config *AdminConfig) error
}
