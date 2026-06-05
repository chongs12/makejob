package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/question/internal/biz"
	"makejob/pkg/mq"
)

// MQConsumer handles MQ messages for the question service.
type MQConsumer struct {
	consumer *mq.Consumer
	uc       *biz.QuestionUseCase
	logger   *log.Helper
}

// NewMQConsumer creates a new MQ consumer for question-related queues.
func NewMQConsumer(url string, uc *biz.QuestionUseCase, logger log.Logger) (*MQConsumer, error) {
	consumer, err := mq.NewConsumer(url)
	if err != nil {
		return nil, fmt.Errorf("failed to create mq consumer: %w", err)
	}

	mc := &MQConsumer{
		consumer: consumer,
		uc:       uc,
		logger:   log.NewHelper(logger),
	}

	// Register RAG sync question handler
	consumer.Register(mq.QueueRAGSyncQuestion, mq.TaskHandlerFunc(mc.handleRAGSyncQuestion))

	return mc, nil
}

// Start starts the MQ consumer.
func (c *MQConsumer) Start(ctx context.Context) error {
	return c.consumer.Start(ctx)
}

// Stop stops the MQ consumer.
func (c *MQConsumer) Stop(ctx context.Context) error {
	return c.consumer.Stop(ctx)
}

// handleRAGSyncQuestion handles RAG sync question messages.
func (c *MQConsumer) handleRAGSyncQuestion(ctx context.Context, msg mq.TaskMessage) error {
	c.logger.Infof("processing RAG sync question message: entity_type=%s, entity_id=%d", msg.EntityType, msg.EntityID)

	// The payload contains the question data to sync to the RAG index.
	// For now, just log and acknowledge the message.
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal RAG sync payload: %w", err)
	}

	c.logger.Infof("RAG sync question completed: entity_id=%d", msg.EntityID)
	return nil
}
