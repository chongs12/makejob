package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"makejob/app/learning_archive/internal/biz"
	"makejob/app/learning_archive/internal/conf"
	"makejob/pkg/mq"
)

// mqPublisher 实现 biz.MQPublisher 接口，负责发布学习档案事件
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

// PublishArchiveWritten 发布档案写入事件
func (p *mqPublisher) PublishArchiveWritten(ctx context.Context, userID uint64, source string, sourceID uint64, weakTopicsAdded, strengthTopicsAdded []string) error {
	payload := mq.ArchiveWrittenPayload{
		UserID:              userID,
		Source:              source,
		SourceID:            sourceID,
		WeakTopicsAdded:     weakTopicsAdded,
		StrengthTopicsAdded: strengthTopicsAdded,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化 archive.written 事件负载失败: %w", err)
	}

	msg := mq.TaskMessage{
		TaskType:   "archive.written",
		EntityType: "learning_archive",
		EntityID:   sourceID,
		Payload:    payloadBytes,
		RetryCount: 3,
		CreatedAt:  time.Now(),
	}

	return p.publisher.Publish(ctx, mq.RoutingKeyArchiveWritten, msg)
}

// Close 关闭发布者连接
func (p *mqPublisher) Close() error {
	return p.publisher.Close()
}
