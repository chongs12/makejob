package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"makejob/app/plan/internal/biz"
	"makejob/app/plan/internal/conf"
	"makejob/pkg/mq"
)

// mqPublisher 实现 biz.MQPublisher 接口，负责发布计划相关 MQ 消息
type mqPublisher struct {
	publisher *mq.Publisher
}

// NewMQPublisher 创建 MQ 消息发布者
func NewMQPublisher(cfg *conf.MQ) (biz.MQPublisher, error) {
	publisher, err := mq.NewPublisher(cfg.URL, cfg.Exchange)
	if err != nil {
		return nil, fmt.Errorf("创建 MQ 发布者失败: %w", err)
	}
	return &mqPublisher{publisher: publisher}, nil
}

// PublishPlanGenerate 发布计划生成 MQ 消息
func (p *mqPublisher) PublishPlanGenerate(ctx context.Context, planID, userID uint64, req *biz.CreatePlanRequest) error {
	payload := mq.PlanGeneratePayload{
		PlanID:            planID,
		UserID:            userID,
		IndustryCode:      req.IndustryCode,
		Goal:              req.Goal,
		DailyHours:        req.DailyHours,
		WeakTopics:        req.WeakTopics,
		Level:             req.Level,
		DurationDays:      req.DurationDays,
		DailyStudyMinutes: req.DailyStudyMinutes,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化计划生成负载失败: %w", err)
	}

	msg := mq.TaskMessage{
		TaskType:   mq.TaskTypePlanGenerate,
		EntityType: "plan",
		EntityID:   planID,
		Payload:    payloadBytes,
		RetryCount: 3,
		CreatedAt:  time.Now(),
	}

	return p.publisher.Publish(ctx, mq.RoutingKeyPlanGenerate, msg)
}

// PublishFeedbackDiagnosis 发布反馈诊断 MQ 消息
func (p *mqPublisher) PublishFeedbackDiagnosis(ctx context.Context, feedbackID, planID, taskID, userID uint64, feedbackText, difficultyFeeling string, problemAreas []string) error {
	payload := mq.FeedbackDiagnosisPayload{
		FeedbackID:        feedbackID,
		PlanID:            planID,
		TaskID:            taskID,
		UserID:            userID,
		FeedbackText:      feedbackText,
		DifficultyFeeling: difficultyFeeling,
		ProblemAreas:      problemAreas,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化反馈诊断负载失败: %w", err)
	}

	msg := mq.TaskMessage{
		TaskType:   mq.TaskTypePlanFeedbackDiagnosis,
		EntityType: "task_feedback",
		EntityID:   feedbackID,
		Payload:    payloadBytes,
		RetryCount: 3,
		CreatedAt:  time.Now(),
	}

	return p.publisher.Publish(ctx, mq.RoutingKeyPlanFeedbackDiagnosis, msg)
}

// Close 关闭发布者连接
func (p *mqPublisher) Close() error {
	return p.publisher.Close()
}
