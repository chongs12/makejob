// Package model 提供数据模型定义
package model

// MessageRole 消息角色枚举
const (
	MessageRoleAI     = "ai"     // AI助手
	MessageRoleUser   = "user"   // 用户
	MessageRoleSystem = "system" // 系统
)

// MessageType 消息类型枚举
const (
	MessageTypeText     = "text"     // 文本消息
	MessageTypeCode     = "code"     // 代码消息
	MessageTypeFeedback = "feedback" // 反馈消息
)

// InterviewMessage 面试消息表
type InterviewMessage struct {
	ID           uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	InterviewID  uint   `json:"interview_id" gorm:"not null;index;comment:面试ID"`
	Role         string `json:"role" gorm:"size:20;not null;comment:角色(ai/user/system)"`
	Content      string `json:"content" gorm:"type:text;not null;comment:消息内容"`
	MessageType  string `json:"message_type" gorm:"size:20;not null;default:'text';comment:消息类型(text/code/feedback)"`
	MetadataJSON string `json:"metadata_json" gorm:"type:text;comment:扩展元数据JSON"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime;comment:创建时间"`

	// 关联关系
	Interview MockInterview `json:"interview,omitempty" gorm:"foreignKey:InterviewID"`
}

// TableName 指定表名
func (InterviewMessage) TableName() string {
	return "interview_messages"
}

// IsFromAI 判断是否来自AI的消息
func (m *InterviewMessage) IsFromAI() bool {
	return m.Role == MessageRoleAI
}

// IsFromUser 判断是否来自用户的消息
func (m *InterviewMessage) IsFromUser() bool {
	return m.Role == MessageRoleUser
}
