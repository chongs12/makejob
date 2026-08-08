package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/learning_archive/internal/biz"
	"makejob/app/learning_archive/internal/conf"
	"makejob/pkg/mq"
)

// MQConsumer 学习档案服务的 MQ 消费者，处理面试完成事件
type MQConsumer struct {
	consumer *mq.Consumer
	uc       *biz.ArchiveUseCase
	logger   *log.Helper
}

// NewMQConsumer 创建学习档案服务的 MQ 消费者
func NewMQConsumer(cfg *conf.MQ, uc *biz.ArchiveUseCase, logger log.Logger) (*MQConsumer, error) {
	consumer, err := mq.NewConsumer(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("创建 MQ 消费者失败: %w", err)
	}

	mc := &MQConsumer{
		consumer: consumer,
		uc:       uc,
		logger:   log.NewHelper(logger),
	}

	// 注册面试完成事件队列处理器
	consumer.Register(mq.QueueLearningArchiveInterviewFinished, mq.TaskHandlerFunc(mc.handleInterviewFinished))

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

// handleInterviewFinished 处理 interview.finished 事件，将面试结果写入学习档案
func (c *MQConsumer) handleInterviewFinished(ctx context.Context, msg mq.TaskMessage) error {
	var payload mq.InterviewFinishedPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("解析 interview.finished 事件负载失败: %w", err)
	}

	c.logger.Infof("处理面试完成事件: interview_id=%d, user_id=%d, score=%.1f, type=%s", payload.InterviewID, payload.UserID, payload.Score, payload.InterviewType)

	processed, err := c.uc.HasInterviewFinishedArchive(ctx, payload.InterviewID, payload.UserID)
	if err == nil && processed {
		return nil
	}
	if err != nil {
		return err
	}

	return c.uc.HandleInterviewFinished(ctx, biz.InterviewFinishedContext{
		InterviewID:       payload.InterviewID,
		UserID:            payload.UserID,
		Score:             payload.Score,
		InterviewType:     payload.InterviewType,
		WeakTopics:        payload.WeakTopics,
		StrengthTopics:    payload.StrengthTopics,
		CodingMistakeTags: payload.CodingMistakeTags,
		Summary:           payload.Summary,
		DurationSeconds:   payload.DurationSeconds,
	})
}
