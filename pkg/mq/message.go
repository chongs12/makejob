package mq

import (
	"encoding/json"
	"time"
)

// TaskMessage MQ 任务消息
type TaskMessage struct {
	TaskType   string          `json:"task_type"`
	EntityType string          `json:"entity_type"`
	EntityID   uint64          `json:"entity_id"`
	Payload    json.RawMessage `json:"payload"`
	RetryCount int             `json:"retry_count"`
	CreatedAt  time.Time       `json:"created_at"` // 消息创建时间，用于检测过期消息
}

// 任务类型常量
const (
	TaskTypePlanFeedbackDiagnosis   = "plan.feedback.diagnosis"
	TaskTypePlanGenerate            = "plan.generate"
	TaskTypeScraperImport           = "scraper.import.questions"
	TaskTypeAdminQuestionPipeline   = "admin.question.pipeline.build"
	TaskTypeInterviewResumeParse    = "interview.resume.parse"
	TaskTypeInterviewReportGenerate = "interview.report.generate"
	TaskTypeInterviewArchivePersist = "interview.archive.persist"
	TaskTypeRAGSyncQuestion         = "rag.sync.question"
)

// 队列名称常量
const (
	QueuePlanFeedbackDiagnosis   = "makejob.async.plan.feedback.diagnosis"
	QueuePlanGenerate            = "makejob.async.plan.generate"
	QueueScraperImport           = "makejob.async.scraper.import.questions"
	QueueAdminQuestionPipeline   = "makejob.async.admin.question.pipeline.build"
	QueueInterviewResumeParse    = "makejob.async.interview.resume.parse"
	QueueInterviewReportGenerate = "makejob.async.interview.report.generate"
	QueueInterviewArchivePersist = "makejob.async.interview.archive.persist"
	QueueRAGSyncQuestion         = "makejob.async.rag.sync.question"
)

// InterviewResumeParsePayload 简历解析负载
type InterviewResumeParsePayload struct {
	InterviewID uint64 `json:"interview_id"`
	UserID      uint64 `json:"user_id"`
	ResumeText  string `json:"resume_text"`
}

// InterviewReportPayload 面试报告负载
type InterviewReportPayload struct {
	InterviewID uint64 `json:"interview_id"`
	UserID      uint64 `json:"user_id"`
}

// InterviewArchivePersistPayload 面试归档负载
type InterviewArchivePersistPayload struct {
	InterviewID uint64 `json:"interview_id"`
	UserID      uint64 `json:"user_id"`
}
