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

	// 学习档案服务队列
	QueueLearningArchiveInterviewFinished = "makejob.events.learning_archive.interview_finished"
)

// 事件路由键常量
const (
	RoutingKeyInterviewResumeParse    = "interview.resume.parse"
	RoutingKeyInterviewReportGenerate = "interview.report.generate"
	RoutingKeyInterviewFinished       = "interview.finished"
	RoutingKeyArchiveWritten          = "archive.written"
	RoutingKeyPlanGenerate            = "plan.generate"
	RoutingKeyPlanFeedbackDiagnosis   = "plan.feedback.diagnosis"
)

// Interview 服务发布到 LearningArchive 消费的队列（统一队列名）
const (
	QueueInterviewFinishedForArchive = QueueLearningArchiveInterviewFinished
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

// InterviewFinishedPayload 面试完成事件负载（由 interview 服务发布，learning_archive 消费）
type InterviewFinishedPayload struct {
	InterviewID    uint64   `json:"interview_id"`
	UserID         uint64   `json:"user_id"`
	Score          float64  `json:"score"`
	WeakTopics     []string `json:"weak_topics"`
	StrengthTopics []string `json:"strength_topics"`
}

// PlanGeneratePayload 计划生成负载
type PlanGeneratePayload struct {
	PlanID            uint64   `json:"plan_id"`
	UserID            uint64   `json:"user_id"`
	IndustryCode      string   `json:"industry_code"`
	Goal              string   `json:"goal"`
	DailyHours        int32    `json:"daily_hours"`
	WeakTopics        []string `json:"weak_topics"`
	Level             string   `json:"level"`
	DurationDays      int32    `json:"duration_days"`
	DailyStudyMinutes int32    `json:"daily_study_minutes"`
}

// FeedbackDiagnosisPayload 反馈诊断负载
type FeedbackDiagnosisPayload struct {
	FeedbackID        uint64   `json:"feedback_id"`
	PlanID            uint64   `json:"plan_id"`
	TaskID            uint64   `json:"task_id"`
	UserID            uint64   `json:"user_id"`
	FeedbackText      string   `json:"feedback_text"`
	DifficultyFeeling string   `json:"difficulty_feeling"`
	ProblemAreas      []string `json:"problem_areas"`
}

// ArchiveWrittenPayload 学习档案写入事件负载（由 learning_archive 服务发布）
type ArchiveWrittenPayload struct {
	UserID              uint64   `json:"user_id"`
	Source              string   `json:"source"`
	SourceID            uint64   `json:"source_id"`
	WeakTopicsAdded     []string `json:"weak_topics_added"`
	StrengthTopicsAdded []string `json:"strength_topics_added"`
}

// PipelineBuildPayload 题目生成流水线负载（由 admin 服务发布，question 消费）
type PipelineBuildPayload struct {
	TaskID           uint64   `json:"task_id"`
	IndustryCode     string   `json:"industry_code"`
	Requirement      string   `json:"requirement"`
	AgentPrompt      string   `json:"agent_prompt"`
	GenerationMode   string   `json:"generation_mode"`
	CandidateCount   int32    `json:"candidate_count"`
	IncludeScraped   bool     `json:"include_scraped"`
	IncludeGenerated bool     `json:"include_generated"`
	Sources          []string `json:"sources"`
}
