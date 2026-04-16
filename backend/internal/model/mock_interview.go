// Package model 提供数据模型定义
package model

import (
	"time"
)

// InterviewStatus 面试状态枚举
const (
	InterviewStatusOngoing   = "ongoing"   // 进行中
	InterviewStatusCompleted = "completed" // 已完成
	InterviewStatusCancelled = "cancelled" // 已取消
)

// MockInterview 模拟面试记录表
type MockInterview struct {
	BaseModel
	UserID         uint       `json:"user_id" gorm:"not null;index;comment:用户ID"`
	IndustryID     uint       `json:"industry_id" gorm:"not null;index;comment:行业ID"`
	Status         string     `json:"status" gorm:"size:20;not null;default:'ongoing';comment:状态(ongoing/completed/cancelled)"`
	Score          float64    `json:"score" gorm:"type:decimal(5,2);comment:面试得分"`
	AIFeedback     string     `json:"ai_feedback" gorm:"type:text;comment:AI反馈评价"`
	TotalQuestions int        `json:"total_questions" gorm:"comment:总题目数"`
	StartedAt      *time.Time `json:"started_at" gorm:"comment:开始时间"`
	EndedAt        *time.Time `json:"ended_at" gorm:"comment:结束时间"`

	// 关联关系
	User     User               `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Industry Industry           `json:"industry,omitempty" gorm:"foreignKey:IndustryID"`
	Messages []InterviewMessage `json:"messages,omitempty" gorm:"foreignKey:InterviewID"`
}

// TableName 指定表名
func (MockInterview) TableName() string {
	return "mock_interviews"
}

// IsOngoing 判断面试是否进行中
func (m *MockInterview) IsOngoing() bool {
	return m.Status == InterviewStatusOngoing
}

// IsCompleted 判断面试是否已完成
func (m *MockInterview) IsCompleted() bool {
	return m.Status == InterviewStatusCompleted
}

// Duration 计算面试时长(分钟)
func (m *MockInterview) Duration() int {
	if m.StartedAt == nil || m.EndedAt == nil {
		return 0
	}
	return int(m.EndedAt.Sub(*m.StartedAt).Minutes())
}
