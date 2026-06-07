package server

import (
	"context"
	"encoding/json"
	"fmt"

	"makejob/app/plan/internal/biz"
	"makejob/app/plan/internal/conf"
	"makejob/pkg/mq"
)

// MQConsumer 实现 kratos/transport.Server，负责 Plan 服务的 MQ 消费。
type MQConsumer struct {
	consumer *mq.Consumer
	uc       *biz.PlanUseCase
}

// planGenerateHandler 处理计划生成消息及最终失败补偿。
type planGenerateHandler struct {
	uc *biz.PlanUseCase
}

// feedbackDiagnosisHandler 处理反馈诊断消息及最终失败补偿。
type feedbackDiagnosisHandler struct {
	uc *biz.PlanUseCase
}

// NewMQConsumer 创建 MQ 消费者
func NewMQConsumer(cfg *conf.MQ, uc *biz.PlanUseCase) (*MQConsumer, error) {
	consumer, err := mq.NewConsumer(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("创建 MQ 消费者失败: %w", err)
	}
	return &MQConsumer{consumer: consumer, uc: uc}, nil
}

// Start 启动消费并注册计划相关处理器。
func (c *MQConsumer) Start(ctx context.Context) error {
	c.consumer.Register(mq.QueuePlanGenerate, &planGenerateHandler{uc: c.uc})
	c.consumer.Register(mq.QueuePlanFeedbackDiagnosis, &feedbackDiagnosisHandler{uc: c.uc})
	return c.consumer.Start(ctx)
}

// Stop 停止消费
func (c *MQConsumer) Stop(ctx context.Context) error {
	return c.consumer.Stop(ctx)
}

// Handle 处理计划生成消息。
func (h *planGenerateHandler) Handle(ctx context.Context, msg mq.TaskMessage) error {
	var payload mq.PlanGeneratePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("解析计划生成负载失败: %w", err)
	}

	req := &biz.CreatePlanRequest{
		UserID:            payload.UserID,
		IndustryCode:      payload.IndustryCode,
		Goal:              payload.Goal,
		DailyHours:        payload.DailyHours,
		WeakTopics:        payload.WeakTopics,
		Level:             payload.Level,
		DurationDays:      payload.DurationDays,
		DailyStudyMinutes: payload.DailyStudyMinutes,
	}
	return h.uc.GeneratePlan(ctx, payload.PlanID, payload.UserID, req)
}

// HandleFinalFailure 在计划生成消息最终失败后落失败状态。
func (h *planGenerateHandler) HandleFinalFailure(ctx context.Context, msg mq.TaskMessage, lastErr error) error {
	var payload mq.PlanGeneratePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}
	return h.uc.MarkPlanGenerateFailed(ctx, payload.PlanID)
}

// Handle 处理反馈诊断消息。
func (h *feedbackDiagnosisHandler) Handle(ctx context.Context, msg mq.TaskMessage) error {
	var payload mq.FeedbackDiagnosisPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("解析反馈诊断负载失败: %w", err)
	}
	return h.uc.ProcessFeedbackDiagnosis(ctx, payload.FeedbackID, payload.FeedbackText, payload.DifficultyFeeling, payload.ProblemAreas)
}

// HandleFinalFailure 在诊断消息最终失败后落失败状态。
func (h *feedbackDiagnosisHandler) HandleFinalFailure(ctx context.Context, msg mq.TaskMessage, lastErr error) error {
	var payload mq.FeedbackDiagnosisPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}
	return h.uc.MarkFeedbackDiagnosisFailed(ctx, payload.FeedbackID)
}
