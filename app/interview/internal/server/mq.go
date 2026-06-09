package server

import (
	"context"
	"encoding/json"
	"fmt"

	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/conf"
	"makejob/pkg/mq"
)

// MQConsumer 实现 kratos/transport.Server，作为 Kratos app 的一个 transport
type MQConsumer struct {
	consumer *mq.Consumer
	uc       *biz.InterviewUseCase
}

// NewMQConsumer 创建 MQ 消费者（由 Wire 调用）
func NewMQConsumer(cfg *conf.MQ, uc *biz.InterviewUseCase) (*MQConsumer, error) {
	consumer, err := mq.NewConsumer(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to create MQ consumer: %w", err)
	}

	return &MQConsumer{consumer: consumer, uc: uc}, nil
}

// Start 启动消费（实现 kratos/transport.Server）
func (c *MQConsumer) Start(ctx context.Context) error {
	// 注册面试相关的队列处理器
	c.consumer.Register(mq.QueueInterviewResumeParse, mq.TaskHandlerFunc(func(ctx context.Context, msg mq.TaskMessage) error {
		var payload mq.InterviewResumeParsePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal resume parse payload: %w", err)
		}
		processed, err := c.uc.IsResumeParsed(ctx, payload.InterviewID)
		if err == nil && processed {
			return nil
		}
		if err != nil {
			return c.uc.ProcessResumeParse(ctx, payload.InterviewID, payload.UserID, payload.ResumeText)
		}
		return c.uc.ProcessResumeParse(ctx, payload.InterviewID, payload.UserID, payload.ResumeText)
	}))

	c.consumer.Register(mq.QueueInterviewReportGenerate, mq.TaskHandlerFunc(func(ctx context.Context, msg mq.TaskMessage) error {
		var payload mq.InterviewReportPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal report payload: %w", err)
		}
		return c.uc.GenerateReport(ctx, payload.InterviewID, payload.UserID)
	}))

	c.consumer.Register(mq.QueueInterviewArchivePersist, mq.TaskHandlerFunc(func(ctx context.Context, msg mq.TaskMessage) error {
		var payload mq.InterviewArchivePersistPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal archive payload: %w", err)
		}
		processed, err := c.uc.HasCodingArchive(ctx, payload.InterviewID, payload.UserID)
		if err == nil && processed {
			return nil
		}
		if err != nil {
			return err
		}
		return c.uc.PersistCodingArchive(ctx, payload.InterviewID, payload.UserID)
	}))

	return c.consumer.Start(ctx)
}

// Stop 停止消费（实现 kratos/transport.Server）
func (c *MQConsumer) Stop(ctx context.Context) error {
	return c.consumer.Stop(ctx)
}
