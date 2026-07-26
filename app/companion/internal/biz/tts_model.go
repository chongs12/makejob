package biz

import (
	"time"

	"gorm.io/gorm"
)

// TTSConfig TTS 配置实体（对齐单体 model.TTSConfig）
type TTSConfig struct {
	ID             uint           `gorm:"primaryKey;autoIncrement"`
	Name           string         `gorm:"size:100;not null;comment:音色名称"`
	Engine         string         `gorm:"size:32;not null;comment:TTS供应商引擎(volcengine/minimax/xiaomi_mimo)"`
	VoiceID        string         `gorm:"size:100;not null;comment:音色ID或说话人ID"`
	AuthConfigJSON string         `gorm:"type:text;comment:鉴权配置JSON"`
	ParamsJSON     string         `gorm:"type:text;comment:额外参数JSON"`
	IsActive       bool           `gorm:"not null;default:true;comment:是否启用"`
	SortOrder      int            `gorm:"not null;default:0;comment:排序顺序"`
	CreatedAt      time.Time      `gorm:"not null;autoCreateTime"`
	UpdatedAt      time.Time      `gorm:"not null;autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (TTSConfig) TableName() string { return "tts_configs" }

// TTS 供应商引擎常量
const (
	TTSEngineVolcengine = "volcengine"
	TTSEngineMinimax    = "minimax"
	TTSEngineXiaomiMIMO = "xiaomi_mimo"
)

// Live2DModel Live2D 模型实体（对齐单体 model.Live2DModel）
type Live2DModel struct {
	ID             uint           `gorm:"primaryKey;autoIncrement"`
	Name           string         `gorm:"size:100;not null;comment:模型名称"`
	Scene          string         `gorm:"size:20;not null;comment:使用场景(interview/companion)"`
	ModelURL       string         `gorm:"size:500;not null;comment:模型文件URL"`
	ThumbnailURL   string         `gorm:"size:500;comment:缩略图URL"`
	ConfigJSON     string         `gorm:"type:text;comment:模型配置JSON"`
	TTSConfigID    *uint          `gorm:"index;comment:绑定的TTS配置ID"`
	IsActive       bool           `gorm:"not null;default:true;comment:是否启用"`
	CreatedAt      time.Time      `gorm:"not null;autoCreateTime"`
	UpdatedAt      time.Time      `gorm:"not null;autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`

	TTSConfig *TTSConfig `gorm:"foreignKey:TTSConfigID"`
}

func (Live2DModel) TableName() string { return "live2d_models" }

// Live2D 场景常量
const (
	Live2DSceneInterview = "interview"
	Live2DSceneCompanion = "companion"
)

// AdminConfig 管理后台配置实体（对齐单体 model.AdminConfig）
type AdminConfig struct {
	ID          uint      `gorm:"primaryKey;autoIncrement"`
	ConfigKey   string    `gorm:"size:100;not null;uniqueIndex;comment:配置键"`
	ConfigValue string    `gorm:"type:text;comment:配置值"`
	CreatedAt   time.Time `gorm:"not null;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"not null;autoUpdateTime"`
}

func (AdminConfig) TableName() string { return "admin_configs" }

// 场景默认 TTS 配置键。必须与 admin 服务写入 admin_configs 的键一致
// （admin 服务用 "tts_default_" + scene，即 tts_default_interview / tts_default_companion）。
// 历史上这里误写成 *_config_id 后缀，导致后台设置的场景默认 TTS 永远查不到。
const (
	TTSDefaultConfigKeyInterview = "tts_default_interview"
	TTSDefaultConfigKeyCompanion = "tts_default_companion"
)
