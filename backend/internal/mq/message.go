// Package mq 提供 RabbitMQ 消息定义与基础设施封装。
package mq

import (
	"encoding/json"
	"time"
)

const (
	// MessageVersionV1 表示当前任务消息结构版本。
	MessageVersionV1 = "v1"

	// TaskTypePlanFeedbackDiagnosis 表示学习任务反馈诊断任务。
	TaskTypePlanFeedbackDiagnosis = "plan.feedback.diagnosis"
	// TaskTypeScraperImport 表示爬虫题目导入任务。
	TaskTypeScraperImport = "scraper.import.questions"
	// TaskTypeAdminQuestionPipeline 表示后台题库流水线生成任务。
	TaskTypeAdminQuestionPipeline = "admin.question.pipeline.build"
	// TaskTypeInterviewResumeParse 表示简历解析任务。
	TaskTypeInterviewResumeParse = "interview.resume.parse"
	// TaskTypeInterviewReportGenerate 表示面试报告生成任务。
	TaskTypeInterviewReportGenerate = "interview.report.generate"
	// TaskTypeInterviewArchivePersist 表示面试编程诊断与学习档案沉淀任务。
	TaskTypeInterviewArchivePersist = "interview.archive.persist"
	// TaskTypePlanGenerate 表示学习计划生成任务。
	TaskTypePlanGenerate = "plan.generate"
)

// TaskMessage 表示统一的异步任务消息结构，供不同服务独立消费。
type TaskMessage struct {
	Version        string          `json:"version"`
	MessageID      string          `json:"message_id"`
	TaskType       string          `json:"task_type"`
	Source         string          `json:"source"`
	TaskID         uint            `json:"task_id,omitempty"`
	EntityType     string          `json:"entity_type,omitempty"`
	EntityID       uint            `json:"entity_id,omitempty"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	Priority       int             `json:"priority,omitempty"`
	Attempt        int             `json:"attempt,omitempty"`
	MaxRetries     int             `json:"max_retries,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

// ScraperImportPayload 表示爬虫导入任务消费时需要的任务标识。
type ScraperImportPayload struct {
	ScraperTaskID uint `json:"scraper_task_id"`
}

// AdminQuestionPipelinePayload 表示后台题目流水线任务消费时需要的任务标识。
type AdminQuestionPipelinePayload struct {
	ScraperTaskID uint `json:"scraper_task_id"`
}

// PlanFeedbackDiagnosisPayload 表示学习任务反馈诊断任务的业务载荷。
type PlanFeedbackDiagnosisPayload struct {
	PlanID     uint `json:"plan_id"`
	TaskID     uint `json:"task_id"`
	FeedbackID uint `json:"feedback_id"`
	UserID     uint `json:"user_id"`
}

// InterviewResumeParsePayload 表示简历解析任务的业务载荷。
type InterviewResumeParsePayload struct {
	InterviewID    uint   `json:"interview_id"`
	UserID         uint   `json:"user_id"`
	IndustryCode   string `json:"industry_code"`
	ResumeText     string `json:"resume_text"`
	JobDescription string `json:"job_description"`
	InterviewMode  string `json:"interview_mode"`
	Live2DModelKey string `json:"live2d_model_key"`
	QuestionCount  int    `json:"question_count"`
	Difficulty     string `json:"difficulty"`
}

// InterviewReportPayload 表示面试报告生成任务的业务载荷。
type InterviewReportPayload struct {
	InterviewID uint   `json:"interview_id"`
	UserID      uint   `json:"user_id"`
	SessionID   string `json:"session_id"`
}

// InterviewArchivePayload 表示面试编程诊断与学习档案沉淀任务的业务载荷。
type InterviewArchivePayload struct {
	InterviewID uint `json:"interview_id"`
	UserID      uint `json:"user_id"`
}

// PlanGeneratePayload 表示学习计划生成任务的业务载荷。
type PlanGeneratePayload struct {
	PlanID       uint            `json:"plan_id"`
	UserID       uint            `json:"user_id"`
	RequestJSON  json.RawMessage `json:"request_json"`
	IndustryCode string          `json:"industry_code"`
}
