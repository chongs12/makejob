package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/conf"
	"makejob/pkg/mq"
)

type mqPublisher struct {
	publisher *mq.Publisher
}

// NewMQPublisher 创建 MQ 消息发布者（由 Wire 调用）
func NewMQPublisher(cfg *conf.MQ) (biz.MQPublisher, error) {
	publisher, err := mq.NewPublisher(cfg.URL, cfg.Exchange)
	if err != nil {
		return nil, fmt.Errorf("failed to create MQ publisher: %w", err)
	}
	return &mqPublisher{publisher: publisher}, nil
}

// PublishInterviewReportGenerate 发布面试报告生成消息
func (p *mqPublisher) PublishInterviewReportGenerate(ctx context.Context, interviewID, userID uint64) error {
	payload := mq.InterviewReportPayload{
		InterviewID: interviewID,
		UserID:      userID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal report payload: %w", err)
	}

	msg := mq.TaskMessage{
		TaskType:   mq.TaskTypeInterviewReportGenerate,
		EntityType: "interview",
		EntityID:   interviewID,
		Payload:    payloadBytes,
		RetryCount: 3,
		CreatedAt:  time.Now(),
	}

	return p.publisher.Publish(ctx, mq.RoutingKeyInterviewReportGenerate, msg)
}

// PublishInterviewResumeParse 发布简历解析消息。
func (p *mqPublisher) PublishInterviewResumeParse(ctx context.Context, interviewID, userID uint64, resumeText string) error {
	payload := mq.InterviewResumeParsePayload{
		InterviewID: interviewID,
		UserID:      userID,
		ResumeText:  resumeText,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal resume parse payload: %w", err)
	}

	msg := mq.TaskMessage{
		TaskType:   mq.TaskTypeInterviewResumeParse,
		EntityType: "interview",
		EntityID:   interviewID,
		Payload:    payloadBytes,
		RetryCount: 3,
		CreatedAt:  time.Now(),
	}

	return p.publisher.Publish(ctx, mq.RoutingKeyInterviewResumeParse, msg)
}

// PublishInterviewFinished 发布面试完成事件（携带面试快照数据）
func (p *mqPublisher) PublishInterviewFinished(ctx context.Context, event biz.InterviewFinishedEvent) error {
	payload := mq.InterviewFinishedPayload{
		InterviewID:       event.InterviewID,
		UserID:            event.UserID,
		Score:             event.Score,
		InterviewType:     event.InterviewType,
		WeakTopics:        event.WeakTopics,
		StrengthTopics:    event.StrengthTopics,
		CodingMistakeTags: event.CodingMistakeTags,
		Summary:           event.Summary,
		DurationSeconds:   event.DurationSeconds,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal finished payload: %w", err)
	}

	msg := mq.TaskMessage{
		TaskType:   "interview.finished",
		EntityType: "interview",
		EntityID:   event.InterviewID,
		Payload:    payloadBytes,
		RetryCount: 3,
		CreatedAt:  time.Now(),
	}

	// 使用面试完成事件队列发布（LearningArchive 消费此队列）
	return p.publisher.Publish(ctx, mq.RoutingKeyInterviewFinished, msg)
}

// Close 关闭发布者连接
func (p *mqPublisher) Close() error {
	return p.publisher.Close()
}
