// Package model 提供数据模型定义
package model

// InterviewCodingEventType 定义编程题过程事件类型。
const (
	InterviewCodingEventTypeCodeSnapshot = "code_snapshot"
	InterviewCodingEventTypeRunCode      = "run_code"
	InterviewCodingEventTypeRunResult    = "run_result"
	InterviewCodingEventTypeSubmitCode   = "submit_code"
	InterviewCodingEventTypeIdleTimeout  = "idle_timeout"
)

// InterviewCodingAttempt 表示一场面试中某道编程题的完整作答记录。
type InterviewCodingAttempt struct {
	BaseModel
	InterviewID      uint   `json:"interview_id" gorm:"not null;index;uniqueIndex:idx_interview_coding_question;comment:面试ID"`
	UserID           uint   `json:"user_id" gorm:"not null;index;comment:用户ID"`
	QuestionIndex    int    `json:"question_index" gorm:"not null;uniqueIndex:idx_interview_coding_question;comment:题目序号(从0开始)"`
	QuestionPrompt   string `json:"question_prompt" gorm:"type:text;comment:题目内容"`
	QuestionTopic    string `json:"question_topic" gorm:"size:100;comment:题目主题"`
	QuestionType     string `json:"question_type" gorm:"size:30;comment:题目类型"`
	QuestionDifficulty string `json:"question_difficulty" gorm:"size:30;comment:题目难度"`
	Language         string `json:"language" gorm:"size:30;comment:编程语言"`
	StarterCode      string `json:"starter_code" gorm:"type:text;comment:起始模板代码"`
	FinalCode        string `json:"final_code" gorm:"type:text;comment:最终提交代码"`
	FinalAnswer      string `json:"final_answer" gorm:"type:text;comment:最终文字说明"`
	ProcessSummary   string `json:"process_summary" gorm:"type:text;comment:过程摘要"`
	DiagnosisJSON    string `json:"diagnosis_json" gorm:"type:text;comment:结构化诊断JSON"`

	// 关联关系
	Interview MockInterview          `json:"interview,omitempty" gorm:"foreignKey:InterviewID"`
	Events    []InterviewCodingEvent `json:"events,omitempty" gorm:"foreignKey:AttemptID"`
}

// TableName 指定编程题作答记录表名。
func (InterviewCodingAttempt) TableName() string {
	return "interview_coding_attempts"
}

// InterviewCodingEvent 表示编程题过程中的一条时间序列事件。
type InterviewCodingEvent struct {
	BaseModel
	AttemptID    uint   `json:"attempt_id" gorm:"not null;index;comment:作答记录ID"`
	Sequence     int    `json:"sequence" gorm:"not null;comment:事件顺序"`
	EventType    string `json:"event_type" gorm:"size:40;not null;index;comment:事件类型"`
	EventTSMS    int64  `json:"event_ts_ms" gorm:"not null;comment:事件时间戳(毫秒)"`
	PayloadJSON  string `json:"payload_json" gorm:"type:text;comment:事件载荷JSON"`

	// 关联关系
	Attempt InterviewCodingAttempt `json:"attempt,omitempty" gorm:"foreignKey:AttemptID"`
}

// TableName 指定编程题过程事件表名。
func (InterviewCodingEvent) TableName() string {
	return "interview_coding_events"
}
