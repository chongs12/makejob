// Package model 提供数据模型定义
package model

// Category 题目分类表，支持多级分类
type Category struct {
	BaseModel
	IndustryID  uint   `json:"industry_id" gorm:"not null;index;comment:所属行业ID"`
	Name        string `json:"name" gorm:"size:100;not null;comment:分类名称"`
	ParentID    *uint  `json:"parent_id" gorm:"index;comment:父分类ID，null表示顶级分类"`
	SortOrder   int    `json:"sort_order" gorm:"not null;default:0;comment:排序顺序"`
	Icon        string `json:"icon" gorm:"size:200;comment:图标URL"`
	Description string `json:"description" gorm:"type:text;comment:分类描述"`

	// 关联关系
	Industry Industry   `json:"industry,omitempty" gorm:"foreignKey:IndustryID"`
	Parent   *Category  `json:"parent,omitempty" gorm:"foreignKey:ParentID"`
	Children []Category `json:"children,omitempty" gorm:"foreignKey:ParentID"`
}

// TableName 指定表名
func (Category) TableName() string {
	return "categories"
}

// IsTopLevel 判断是否为顶级分类
func (c *Category) IsTopLevel() bool {
	return c.ParentID == nil
}
