// Package model 提供数据模型定义
package model

// AIPreset AI 配置预设表。
type AIPreset struct {
	BaseModel
	Name       string `json:"name" gorm:"size:100;not null;uniqueIndex;comment:预设名称"`
	ConfigJSON string `json:"config_json" gorm:"type:text;not null;comment:完整 AI 配置快照 JSON"`
	IsActive   bool   `json:"is_active" gorm:"not null;default:false;comment:是否为当前生效预设"`
}

// TableName 指定表名。
func (AIPreset) TableName() string {
	return "ai_presets"
}
