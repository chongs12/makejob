package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	adminv1 "makejob/api/makejob/admin/v1"
	"makejob/app/admin/internal/conf"
	"makejob/pkg/mq"
)

// MqPublisher 封装 Admin 侧异步题目流水线消息发布能力。
type MqPublisher struct {
	publisher *mq.Publisher
}

// NewMQPublisher 创建 Admin 服务使用的 RabbitMQ 发布器。
func NewMQPublisher(cfg *conf.MQ) (*MqPublisher, error) {
	if cfg == nil || cfg.URL == "" {
		return nil, fmt.Errorf("mq config is required")
	}
	publisher, err := mq.NewPublisher(cfg.URL, cfg.Exchange)
	if err != nil {
		return nil, fmt.Errorf("failed to create MQ publisher: %w", err)
	}
	return &MqPublisher{publisher: publisher}, nil
}

// PublishQuestionPipelineBuild 发布题目流水线构建任务到 question 服务消费队列。
func (p *MqPublisher) PublishQuestionPipelineBuild(ctx context.Context, taskID uint64, req *adminv1.GenerateQuestionPipelineRequest) error {
	payload := mq.PipelineBuildPayload{
		TaskID:           taskID,
		IndustryCode:     req.GetIndustryCode(),
		Requirement:      req.GetRequirement(),
		AgentPrompt:      req.GetAgentPrompt(),
		GenerationMode:   req.GetGenerationMode(),
		CandidateCount:   req.GetCandidateCount(),
		IncludeScraped:   req.GetIncludeScraped(),
		IncludeGenerated: req.GetIncludeGenerated(),
		Sources:          append([]string(nil), req.GetSources()...),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal question pipeline payload: %w", err)
	}
	msg := mq.TaskMessage{
		TaskType:   mq.TaskTypeAdminQuestionPipeline,
		EntityType: "scraper_task",
		EntityID:   taskID,
		Payload:    payloadBytes,
		RetryCount: 5,
		CreatedAt:  time.Now(),
	}
	return p.publisher.Publish(ctx, mq.TaskTypeAdminQuestionPipeline, msg)
}

// PublishScraperImport 发布爬虫异步导入任务到 question 服务消费队列。
func (p *MqPublisher) PublishScraperImport(ctx context.Context, taskID uint64, payload []byte) error {
	msg := mq.TaskMessage{
		TaskType:   mq.TaskTypeScraperImport,
		EntityType: "scraper_task",
		EntityID:   taskID,
		Payload:    payload,
		RetryCount: 3,
		CreatedAt:  time.Now(),
	}
	return p.publisher.Publish(ctx, mq.TaskTypeScraperImport, msg)
}

// Close 关闭 RabbitMQ 发布器连接。
func (p *MqPublisher) Close() error {
	if p == nil || p.publisher == nil {
		return nil
	}
	return p.publisher.Close()
}
