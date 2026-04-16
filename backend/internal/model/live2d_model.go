// Package model 提供数据模型定义
package model

// Live2DScene 使用场景枚举
const (
	Live2DSceneInterview = "interview" // 面试场景
	Live2DSceneCompanion = "companion" // 陪伴场景
)

// Live2DModel Live2D模型配置表
type Live2DModel struct {
	BaseModel
	Name         string `json:"name" gorm:"size:100;not null;comment:模型名称"`
	IndustryID   uint   `json:"industry_id" gorm:"index;comment:所属行业ID，0表示通用"`
	Scene        string `json:"scene" gorm:"size:20;not null;comment:使用场景(interview/companion)"`
	ModelURL     string `json:"model_url" gorm:"size:500;not null;comment:模型文件URL"`
	ThumbnailURL string `json:"thumbnail_url" gorm:"size:500;comment:缩略图URL"`
	ConfigJSON   string `json:"config_json" gorm:"type:text;comment:模型配置JSON"`
	IsActive     bool   `json:"is_active" gorm:"not null;default:true;comment:是否启用"`

	// 关联关系
	Industry Industry `json:"industry,omitempty" gorm:"foreignKey:IndustryID"`
}

// TableName 指定表名
func (Live2DModel) TableName() string {
	return "live2d_models"
}

// IsGeneric 判断是否为通用模型
func (l *Live2DModel) IsGeneric() bool {
	return l.IndustryID == 0
}
