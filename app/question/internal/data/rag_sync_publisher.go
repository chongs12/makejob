package data

import (
	"context"
	"encoding/json"
	"fmt"

	"makejob/app/question/internal/biz"
	"makejob/pkg/mq"
)

// ragSyncPublisher 实现 biz.RAGSyncPublisher，通过 MQ 发布题目变更事件
type ragSyncPublisher struct {
	publisher *mq.Publisher
}

// NewRAGSyncPublisher 创建 RAG 同步事件发布器
func NewRAGSyncPublisher(publisher *mq.Publisher) biz.RAGSyncPublisher {
	return &ragSyncPublisher{publisher: publisher}
}

// PublishQuestionChanged 发布题目变更消息到 RAG 同步队列
func (p *ragSyncPublisher) PublishQuestionChanged(ctx context.Context, questionID uint64, action string, content string, metadata map[string]string) error {
	payload := map[string]interface{}{
		"question_id": questionID,
		"action":      action,
		"content":     content,
		"metadata":    metadata,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal RAG sync payload: %w", err)
	}

	msg := mq.TaskMessage{
		TaskType:   mq.TaskTypeRAGSyncQuestion,
		EntityType: "question",
		EntityID:   questionID,
		Payload:    payloadBytes,
		RetryCount: 3,
	}

	return p.publisher.Publish(ctx, mq.TaskTypeRAGSyncQuestion, msg)
}
