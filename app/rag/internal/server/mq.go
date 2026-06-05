package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/rag/internal/biz"
	"makejob/pkg/mq"
)

// QuestionChangedPayload 题目变更消息负载
type QuestionChangedPayload struct {
	QuestionID uint64         `json:"question_id"`
	Action     string         `json:"action"`
	Content    string         `json:"content"`
	Metadata   map[string]any `json:"metadata"`
}

// MQConsumer RAG 服务的 MQ 消费者，处理题目同步消息
type MQConsumer struct {
	consumer    *mq.Consumer
	syncHandler *biz.SyncHandler
	logger      *log.Helper
}

// NewMQConsumer 创建 RAG 服务的 MQ 消费者
func NewMQConsumer(url string, syncHandler *biz.SyncHandler, logger log.Logger) (*MQConsumer, error) {
	consumer, err := mq.NewConsumer(url)
	if err != nil {
		return nil, fmt.Errorf("创建 MQ 消费者失败: %w", err)
	}

	mc := &MQConsumer{
		consumer:    consumer,
		syncHandler: syncHandler,
		logger:      log.NewHelper(logger),
	}

	// 注册 RAG 题目同步队列处理器
	consumer.Register(mq.QueueRAGSyncQuestion, mq.TaskHandlerFunc(mc.handleRAGSyncQuestion))

	return mc, nil
}

// Start 启动消费（实现 kratos/transport.Server 接口）
func (c *MQConsumer) Start(ctx context.Context) error {
	return c.consumer.Start(ctx)
}

// Stop 停止消费（实现 kratos/transport.Server 接口）
func (c *MQConsumer) Stop(ctx context.Context) error {
	return c.consumer.Stop(ctx)
}

// handleRAGSyncQuestion 处理 RAG 题目同步消息
func (c *MQConsumer) handleRAGSyncQuestion(ctx context.Context, msg mq.TaskMessage) error {
	c.logger.Infof("收到 RAG 题目同步消息: entity_type=%s, entity_id=%d", msg.EntityType, msg.EntityID)

	var payload QuestionChangedPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("解析 RAG 同步消息失败: %w", err)
	}

	// 若消息中未携带 question_id，使用 entity_id 兜底
	if payload.QuestionID == 0 {
		payload.QuestionID = msg.EntityID
	}

	return c.syncHandler.HandleQuestionChanged(
		ctx,
		payload.QuestionID,
		payload.Action,
		payload.Content,
		payload.Metadata,
	)
}
