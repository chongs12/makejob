// Package model 提供数据模型定义
package model

import (
	"time"
)

// TaskType 任务类型枚举
const (
	TaskTypeStudy     = "study"     // 学习
	TaskTypePractice  = "practice"  // 练习
	TaskTypeInterview = "interview" // 面试
	TaskTypeReview    = "review"    // 复习
)

// TaskStatus 任务状态枚举
const (
	TaskStatusPending    = "pending"     // 待完成
	TaskStatusInProgress = "in_progress" // 进行中
	TaskStatusCompleted  = "completed"   // 已完成
	TaskStatusSkipped    = "skipped"     // 已跳过
)

// LearningTask 学习任务表
type LearningTask struct {
	BaseModel
	PlanID      uint       `json:"plan_id" gorm:"not null;index;comment:计划ID"`
	Title       string     `json:"title" gorm:"size:200;not null;comment:任务标题"`
	Description string     `json:"description" gorm:"type:text;comment:任务描述"`
	TaskType    string     `json:"task_type" gorm:"size:20;not null;comment:任务类型(study/practice/interview/review)"`
	TargetID    *uint      `json:"target_id" gorm:"comment:关联目标ID(题目或面试ID)"`
	Status      string     `json:"status" gorm:"size:20;not null;default:'pending';comment:状态(pending/in_progress/completed/skipped)"`
	DueDate     *time.Time `json:"due_date" gorm:"comment:截止日期"`
	CompletedAt *time.Time `json:"completed_at" gorm:"comment:完成时间"`
	SortOrder   int        `json:"sort_order" gorm:"not null;default:0;comment:排序顺序"`

	// 关联关系
	Plan LearningPlan `json:"plan,omitempty" gorm:"foreignKey:PlanID"`
}

// TableName 指定表名
func (LearningTask) TableName() string {
	return "learning_tasks"
}

// IsCompleted 判断任务是否已完成
func (t *LearningTask) IsCompleted() bool {
	return t.Status == TaskStatusCompleted
}

// IsOverdue 判断任务是否已逾期
func (t *LearningTask) IsOverdue() bool {
	if t.DueDate == nil || t.IsCompleted() {
		return false
	}
	return time.Now().After(*t.DueDate)
}
