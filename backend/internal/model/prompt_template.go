// Package model 提供数据模型定义
package model

// PromptScene Prompt场景枚举
const (
	PromptSceneInterview = "interview" // 面试场景
	PromptSceneCompanion = "companion" // 陪伴场景
	PromptSceneQuiz      = "quiz"      // 刷题场景
	PromptScenePlan      = "plan"      // 学习计划场景
)

// PromptTemplate Prompt模板表
type PromptTemplate struct {
	BaseModel
	IndustryID      *uint  `json:"industry_id" gorm:"index;comment:所属行业ID，NULL表示通用"`
	Name            string `json:"name" gorm:"size:100;not null;comment:模板名称"`
	Scene           string `json:"scene" gorm:"size:20;not null;comment:使用场景(interview/companion/quiz/plan)"`
	TemplateContent string `json:"template_content" gorm:"type:text;not null;comment:模板内容"`
	Variables       string `json:"variables" gorm:"type:text;comment:模板变量说明JSON"`
	IsActive        bool   `json:"is_active" gorm:"not null;default:true;comment:是否启用"`

	// 关联关系
	Industry Industry `json:"industry,omitempty" gorm:"foreignKey:IndustryID"`
}

// TableName 指定表名
func (PromptTemplate) TableName() string {
	return "prompt_templates"
}

// IsGeneric 判断是否为通用模板(不绑定特定行业)
func (p *PromptTemplate) IsGeneric() bool {
	return p.IndustryID == nil || *p.IndustryID == 0
}
