// Package mq 提供 RabbitMQ 消息定义与基础设施封装。
package mq

import "time"

const (
	// TasksExchangeName 表示业务任务交换机名称。
	TasksExchangeName = "makejob.tasks.topic"
	// RetryExchangeName 表示重试交换机名称。
	RetryExchangeName = "makejob.tasks.retry"
	// DeadLetterExchangeName 表示死信交换机名称。
	DeadLetterExchangeName = "makejob.tasks.dlx"
)

// QueueSpec 描述单类任务对应的队列、路由和重试策略。
type QueueSpec struct {
	TaskType    string
	QueueName   string
	RoutingKey  string
	RetryDelay  time.Duration
	MaxRetries  int
	Description string
}

// RetryQueueName 返回当前业务队列对应的重试队列名称。
func (s QueueSpec) RetryQueueName() string {
	return s.QueueName + ".retry"
}

// DeadLetterQueueName 返回当前业务队列对应的死信队列名称。
func (s QueueSpec) DeadLetterQueueName() string {
	return s.QueueName + ".dlq"
}

// DefaultQueueSpecs 返回当前单体阶段以及后续微服务拆分共用的标准队列拓扑。
func DefaultQueueSpecs() []QueueSpec {
	return []QueueSpec{
		buildQueueSpec(TaskTypePlanFeedbackDiagnosis, "makejob.async.plan.feedback.diagnosis", "plan.feedback.diagnosis", 30*time.Second, 3, "学习任务反馈诊断"),
		buildQueueSpec(TaskTypeScraperImport, "makejob.async.scraper.import.questions", "scraper.import.questions", 20*time.Second, 5, "题库爬虫导入"),
		buildQueueSpec(TaskTypeAdminQuestionPipeline, "makejob.async.admin.question.pipeline.build", "admin.question.pipeline.build", 30*time.Second, 5, "后台题目流水线"),
		buildQueueSpec(TaskTypeInterviewResumeParse, "makejob.async.interview.resume.parse", "interview.resume.parse", 30*time.Second, 3, "简历解析"),
		buildQueueSpec(TaskTypeInterviewReportGenerate, "makejob.async.interview.report.generate", "interview.report.generate", 45*time.Second, 3, "面试报告生成"),
		buildQueueSpec(TaskTypeInterviewArchivePersist, "makejob.async.interview.archive.persist", "interview.archive.persist", 45*time.Second, 3, "面试编程诊断与学习档案沉淀"),
		buildQueueSpec(TaskTypePlanGenerate, "makejob.async.plan.generate", "plan.generate", 30*time.Second, 3, "学习计划生成"),
	}
}

// QueueSpecByTaskType 根据任务类型返回对应队列配置。
func QueueSpecByTaskType(taskType string) (QueueSpec, bool) {
	for _, spec := range DefaultQueueSpecs() {
		if spec.TaskType == taskType {
			return spec, true
		}
	}
	return QueueSpec{}, false
}

// QueueSpecByQueueName 根据业务队列名称返回对应队列配置。
func QueueSpecByQueueName(queueName string) (QueueSpec, bool) {
	for _, spec := range DefaultQueueSpecs() {
		if spec.QueueName == queueName {
			return spec, true
		}
	}
	return QueueSpec{}, false
}

// buildQueueSpec 统一构造标准队列配置，避免不同任务的命名规则分叉。
func buildQueueSpec(taskType string, queueName string, routingKey string, retryDelay time.Duration, maxRetries int, description string) QueueSpec {
	return QueueSpec{
		TaskType:    taskType,
		QueueName:   queueName,
		RoutingKey:  routingKey,
		RetryDelay:  retryDelay,
		MaxRetries:  maxRetries,
		Description: description,
	}
}
