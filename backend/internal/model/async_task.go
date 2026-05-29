// Package model 提供数据模型定义
package model

import "time"

const (
	// AsyncTaskStatusPending 表示任务已入库但尚未成功投递到消息队列。
	AsyncTaskStatusPending = "pending"
	// AsyncTaskStatusQueued 表示任务已经成功投递到消息队列，等待消费者处理。
	AsyncTaskStatusQueued = "queued"
	// AsyncTaskStatusRunning 表示任务已被消费者领取并开始执行。
	AsyncTaskStatusRunning = "running"
	// AsyncTaskStatusSucceeded 表示任务处理成功并已持久化结果。
	AsyncTaskStatusSucceeded = "succeeded"
	// AsyncTaskStatusFailed 表示任务本次执行失败，可能仍会被重试。
	AsyncTaskStatusFailed = "failed"
	// AsyncTaskStatusDead 表示任务超过最大重试次数，已经进入死信状态。
	AsyncTaskStatusDead = "dead"
)

// AsyncTask 通用异步任务记录，承接 RabbitMQ 投递前后的状态审计与幂等控制。
type AsyncTask struct {
	BaseModel
	TaskType       string     `json:"task_type" gorm:"size:100;not null;index;comment:任务类型"`
	Source         string     `json:"source" gorm:"size:100;not null;default:'system';comment:任务来源"`
	Status         string     `json:"status" gorm:"size:20;not null;default:'pending';index;comment:任务状态"`
	QueueName      string     `json:"queue_name" gorm:"size:150;not null;comment:目标队列名称"`
	RoutingKey     string     `json:"routing_key" gorm:"size:150;not null;comment:投递路由键"`
	EntityType     string     `json:"entity_type" gorm:"size:100;comment:关联实体类型"`
	EntityID       uint       `json:"entity_id" gorm:"default:0;index;comment:关联实体ID"`
	IdempotencyKey string     `json:"idempotency_key" gorm:"size:191;uniqueIndex;comment:幂等键"`
	PayloadJSON    string     `json:"payload_json" gorm:"type:text;comment:任务载荷"`
	ResultJSON     string     `json:"result_json" gorm:"type:text;comment:任务结果"`
	RetryCount     int        `json:"retry_count" gorm:"default:0;comment:已执行次数"`
	MaxRetries     int        `json:"max_retries" gorm:"default:3;comment:最大执行次数"`
	PublishedAt    *time.Time `json:"published_at,omitempty" gorm:"comment:成功投递时间"`
	StartedAt      *time.Time `json:"started_at,omitempty" gorm:"comment:开始执行时间"`
	FinishedAt     *time.Time `json:"finished_at,omitempty" gorm:"comment:结束执行时间"`
	ErrorMsg       string     `json:"error_msg" gorm:"type:text;comment:失败原因"`
}

// TableName 指定异步任务表名。
func (AsyncTask) TableName() string {
	return "async_tasks"
}
