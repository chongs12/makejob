// Package mq 提供 RabbitMQ 消息定义与基础设施封装。
package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"makejob-backend/internal/config"
	applogger "makejob-backend/pkg/logger"
)

// TaskPublisher 定义异步任务消息发布器接口。
type TaskPublisher interface {
	PublishTask(ctx context.Context, routingKey string, message TaskMessage) error
	Close() error
}

// rabbitTaskPublisher 提供基于 RabbitMQ 的任务发布器实现。
type rabbitTaskPublisher struct {
	cfg     config.RabbitMQConfig
	specs   []QueueSpec
	conn    *amqp.Connection
	channel *amqp.Channel
	confirm <-chan amqp.Confirmation
	mu      sync.Mutex
}

// NewTaskPublisher 创建 RabbitMQ 任务发布器，并在首次发布前声明标准拓扑。
func NewTaskPublisher(cfg config.RabbitMQConfig, specs []QueueSpec) (TaskPublisher, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	publisher := &rabbitTaskPublisher{
		cfg:   cfg,
		specs: specs,
	}
	if err := publisher.ensureChannelLocked(); err != nil {
		return nil, err
	}
	applogger.Info("RabbitMQ 任务发布器已就绪", zap.String("url", cfg.URL))
	return publisher, nil
}

// PublishTask 序列化并投递统一任务消息，失败时会自动重建连接后重试一次。
func (p *rabbitTaskPublisher) PublishTask(ctx context.Context, routingKey string, message TaskMessage) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化任务消息失败: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureChannelLocked(); err != nil {
		return err
	}
	if err := p.publishLocked(ctx, routingKey, body); err != nil {
		p.closeLocked()
		if err := p.ensureChannelLocked(); err != nil {
			return err
		}
		if retryErr := p.publishLocked(ctx, routingKey, body); retryErr != nil {
			return retryErr
		}
	}

	applogger.Info("任务已发布",
		zap.String("task_type", message.TaskType),
		zap.Uint("task_id", message.TaskID),
		zap.String("message_id", message.MessageID),
		zap.String("routing_key", routingKey),
	)
	return nil
}

// Close 关闭发布器持有的 RabbitMQ 连接与通道。
func (p *rabbitTaskPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
	return nil
}

// ensureChannelLocked 确保当前发布器持有可用通道，并完成拓扑声明。
func (p *rabbitTaskPublisher) ensureChannelLocked() error {
	if p.channel != nil && !p.channel.IsClosed() && p.conn != nil && !p.conn.IsClosed() {
		return nil
	}

	conn, err := amqp.DialConfig(p.cfg.URL, amqp.Config{
		Properties: amqp.Table{"connection_name": "makejob-task-publisher"},
	})
	if err != nil {
		return fmt.Errorf("连接 RabbitMQ 失败: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("创建 RabbitMQ 发布通道失败: %w", err)
	}
	if err := declareTopology(channel, p.specs); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return err
	}
	if p.cfg.PublisherConfirm {
		if err := channel.Confirm(false); err != nil {
			_ = channel.Close()
			_ = conn.Close()
			return fmt.Errorf("开启发布确认失败: %w", err)
		}
		p.confirm = channel.NotifyPublish(make(chan amqp.Confirmation, 1))
	} else {
		p.confirm = nil
	}

	p.conn = conn
	p.channel = channel
	return nil
}

// publishLocked 在持有互斥锁的前提下执行实际消息投递。
func (p *rabbitTaskPublisher) publishLocked(ctx context.Context, routingKey string, body []byte) error {
	if err := p.channel.PublishWithContext(ctx, TasksExchangeName, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	}); err != nil {
		return fmt.Errorf("发布任务消息失败: %w", err)
	}
	if p.confirm == nil {
		return nil
	}

	select {
	case confirmation, ok := <-p.confirm:
		if !ok {
			return fmt.Errorf("发布确认通道已关闭")
		}
		if !confirmation.Ack {
			return fmt.Errorf("RabbitMQ 未确认消息投递")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// closeLocked 关闭当前持有的通道和连接，供重连或退出时复用。
func (p *rabbitTaskPublisher) closeLocked() {
	if p.channel != nil {
		_ = p.channel.Close()
		p.channel = nil
	}
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
	p.confirm = nil
}
