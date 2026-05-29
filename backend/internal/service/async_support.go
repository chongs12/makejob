// Package service 提供业务逻辑层实现
package service

import (
	"time"

	"github.com/google/uuid"

	"makejob-backend/internal/mq"
	"makejob-backend/internal/repository"
)

// AsyncDispatchOption 描述服务接入 RabbitMQ 时需要的可选异步依赖。
type AsyncDispatchOption struct {
	Enabled       bool
	Publisher     mq.TaskPublisher
	AsyncTaskRepo repository.AsyncTaskRepository
}

// buildAsyncTaskMessage 构造统一异步任务消息，保证不同生产者产生相同消息格式。
func buildAsyncTaskMessage(taskType string, taskID uint, entityType string, entityID uint, source string, idempotencyKey string, payload []byte) mq.TaskMessage {
	return mq.TaskMessage{
		Version:        mq.MessageVersionV1,
		MessageID:      uuid.NewString(),
		TaskType:       taskType,
		Source:         source,
		TaskID:         taskID,
		EntityType:     entityType,
		EntityID:       entityID,
		IdempotencyKey: idempotencyKey,
		Priority:       0,
		Attempt:        0,
		MaxRetries:     0,
		CreatedAt:      time.Now(),
		Payload:        payload,
	}
}
