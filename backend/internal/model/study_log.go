// Package model 提供数据模型定义
package model

import "time"

// StudyLog 表示用户某一天在学习陪伴链路中的执行摘要。
type StudyLog struct {
	BaseModel
	UserID              uint      `json:"user_id" gorm:"not null;index;uniqueIndex:idx_user_log_date;comment:用户ID"`
	PlanID              *uint     `json:"plan_id" gorm:"index;comment:关联学习计划ID"`
	LogDate             time.Time `json:"log_date" gorm:"type:date;not null;uniqueIndex:idx_user_log_date;comment:日志日期"`
	Summary             string    `json:"summary" gorm:"type:text;comment:当日学习摘要"`
	FocusTaskTitle      string    `json:"focus_task_title" gorm:"size:200;comment:当前聚焦任务标题"`
	CompletedCount      int       `json:"completed_count" gorm:"not null;default:0;comment:当日完成任务数"`
	SkippedCount        int       `json:"skipped_count" gorm:"not null;default:0;comment:当日跳过任务数"`
	CompletedTitlesJSON string    `json:"completed_titles_json" gorm:"type:text;comment:完成任务标题JSON"`
	SkippedTitlesJSON   string    `json:"skipped_titles_json" gorm:"type:text;comment:跳过任务标题JSON"`
	LatestActionText    string    `json:"latest_action_text" gorm:"type:text;comment:最新执行动作说明"`

	// 关联关系
	User User          `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Plan *LearningPlan `json:"plan,omitempty" gorm:"foreignKey:PlanID"`
}

// TableName 指定学习日志表名。
func (StudyLog) TableName() string {
	return "study_logs"
}
