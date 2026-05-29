// Package model 提供数据模型定义
package model

import (
	"time"
)

// PlanStatus 学习计划状态枚举
const (
	PlanStatusGenerating = "generating" // 生成中
	PlanStatusActive    = "active"    // 进行中
	PlanStatusCompleted = "completed" // 已完成
	PlanStatusPaused    = "paused"    // 已暂停
	PlanStatusAbandoned = "abandoned" // 已放弃
)

// LearningPlan 学习计划表
type LearningPlan struct {
	BaseModel
	UserID         uint       `json:"user_id" gorm:"not null;index;comment:用户ID"`
	IndustryID     uint       `json:"industry_id" gorm:"not null;index;comment:行业ID"`
	Title          string     `json:"title" gorm:"size:200;not null;comment:计划标题"`
	Description    string     `json:"description" gorm:"type:text;comment:计划描述"`
	Phase          string     `json:"phase" gorm:"size:20;not null;default:'foundation';comment:当前学习阶段(foundation/drill/review/mock)"`
	PhaseGoal      string     `json:"phase_goal" gorm:"type:text;comment:当前阶段目标"`
	PlanJSON       string     `json:"plan_json" gorm:"type:text;comment:计划内容JSON"`
	Status         string     `json:"status" gorm:"size:20;not null;default:'active';comment:状态(active/completed/paused/abandoned)"`
	TotalTasks     int        `json:"total_tasks" gorm:"not null;default:0;comment:总任务数"`
	CompletedTasks int        `json:"completed_tasks" gorm:"not null;default:0;comment:已完成任务数"`
	StartDate      *time.Time `json:"start_date" gorm:"comment:开始日期"`
	EndDate        *time.Time `json:"end_date" gorm:"comment:结束日期"`

	// 关联关系
	User     User           `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Industry Industry       `json:"industry,omitempty" gorm:"foreignKey:IndustryID"`
	Tasks    []LearningTask `json:"tasks,omitempty" gorm:"foreignKey:PlanID"`
}

// TableName 指定表名
func (LearningPlan) TableName() string {
	return "learning_plans"
}

// Progress 计算学习进度(百分比)
func (p *LearningPlan) Progress() int {
	if p.TotalTasks == 0 {
		return 0
	}
	return int(float64(p.CompletedTasks) / float64(p.TotalTasks) * 100)
}

// IsActive 判断计划是否进行中
func (p *LearningPlan) IsActive() bool {
	return p.Status == PlanStatusActive
}

// IsGenerating 判断计划是否仍处于异步生成阶段。
func (p *LearningPlan) IsGenerating() bool {
	return p.Status == PlanStatusGenerating
}
