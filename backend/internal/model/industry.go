// Package model 提供数据模型定义
package model

// Industry 行业表
type Industry struct {
	BaseModel
	Code        string `json:"code" gorm:"size:50;not null;uniqueIndex;comment:行业代码"`
	Name        string `json:"name" gorm:"size:100;not null;comment:行业名称"`
	Description string `json:"description" gorm:"type:text;comment:行业描述"`
	Icon        string `json:"icon" gorm:"size:200;comment:图标URL"`
	IsActive    bool   `json:"is_active" gorm:"not null;default:true;comment:是否启用"`
	SortOrder   int    `json:"sort_order" gorm:"not null;default:0;comment:排序顺序"`
}

// TableName 指定表名
func (Industry) TableName() string {
	return "industries"
}
