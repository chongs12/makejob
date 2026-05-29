// Package mq 提供 RabbitMQ 消息定义与基础设施封装。
package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"makejob-backend/internal/config"
	applogger "makejob-backend/pkg/logger"
)

// TaskHandler 定义单条任务消息的消费处理接口。
type TaskHandler interface {
	Handle(ctx context.Context, message TaskMessage) error
}

// TaskHandlerFunc 适配普通函数为任务处理器接口。
type TaskHandlerFunc func(ctx context.Context, message TaskMessage) error

// Handle 执行函数式任务处理逻辑。
func (f TaskHandlerFunc) Handle(ctx context.Context, message TaskMessage) error {
	return f(ctx, message)
}

// Consumer 提供 RabbitMQ 队列消费与失败重试能力。
type Consumer struct {
	cfg      config.RabbitMQConfig
	specs    []QueueSpec
	handlers map[string]TaskHandler
}

// NewConsumer 创建 RabbitMQ 消费器，按业务队列名绑定对应处理器。
func NewConsumer(cfg config.RabbitMQConfig, specs []QueueSpec, handlers map[string]TaskHandler) *Consumer {
	return &Consumer{
		cfg:      cfg,
		specs:    specs,
		handlers: handlers,
	}
}

// Start 持续消费已绑定队列，在连接断开时按配置自动重连。
func (c *Consumer) Start(ctx context.Context) error {
	if !c.cfg.Enabled || len(c.handlers) == 0 {
		return nil
	}

	reconnectDelay := time.Duration(c.cfg.ReconnectSeconds) * time.Second
	if reconnectDelay <= 0 {
		reconnectDelay = 5 * time.Second
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := c.consumeSession(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		applogger.Warn("RabbitMQ 消费会话结束，准备重连", zap.Error(err))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(reconnectDelay):
		}
	}
}

// consumeSession 建立一轮完整的连接与消费会话，直到连接断开或上下文取消。
func (c *Consumer) consumeSession(ctx context.Context) error {
	conn, err := amqp.DialConfig(c.cfg.URL, amqp.Config{
		Properties: amqp.Table{"connection_name": "makejob-task-consumer"},
	})
	if err != nil {
		return fmt.Errorf("连接 RabbitMQ 失败: %w", err)
	}
	defer conn.Close()

	applogger.Info("RabbitMQ 消费会话已建立", zap.String("url", c.cfg.URL))

	setupChannel, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("创建 RabbitMQ 初始化通道失败: %w", err)
	}
	if err := declareTopology(setupChannel, c.specs); err != nil {
		_ = setupChannel.Close()
		return err
	}
	_ = setupChannel.Close()

	sessionErrCh := make(chan error, 1)
	channelClosers := make([]func(), 0, len(c.handlers))

	for queueName, handler := range c.handlers {
		spec, ok := QueueSpecByQueueName(queueName)
		if !ok {
			return fmt.Errorf("未找到队列配置: %s", queueName)
		}
		channel, deliveries, err := c.openConsumerChannel(conn, spec)
		if err != nil {
			return err
		}
		channelClosers = append(channelClosers, func() {
			_ = channel.Close()
		})
		applogger.Info("开始消费队列", zap.String("queue", spec.QueueName), zap.String("routing_key", spec.RoutingKey))
		go c.consumeQueue(ctx, channel, spec, handler, deliveries, sessionErrCh)
	}

	notifyClose := conn.NotifyClose(make(chan *amqp.Error, 1))
	defer closeConsumerChannels(channelClosers)

	select {
	case <-ctx.Done():
		return nil
	case err := <-sessionErrCh:
		return err
	case connErr := <-notifyClose:
		if connErr == nil {
			return fmt.Errorf("RabbitMQ 连接已关闭")
		}
		return connErr
	}
}

// openConsumerChannel 为单个业务队列打开独立通道，避免多个消费者共享通道导致确认逻辑互相影响。
func (c *Consumer) openConsumerChannel(conn *amqp.Connection, spec QueueSpec) (*amqp.Channel, <-chan amqp.Delivery, error) {
	channel, err := conn.Channel()
	if err != nil {
		return nil, nil, fmt.Errorf("创建消费通道失败(%s): %w", spec.QueueName, err)
	}

	prefetch := c.cfg.PrefetchCount
	if prefetch <= 0 {
		prefetch = 1
	}
	if err := channel.Qos(prefetch, 0, false); err != nil {
		_ = channel.Close()
		return nil, nil, fmt.Errorf("设置消费预取失败(%s): %w", spec.QueueName, err)
	}

	deliveries, err := channel.Consume(spec.QueueName, "", false, false, false, false, nil)
	if err != nil {
		_ = channel.Close()
		return nil, nil, fmt.Errorf("订阅业务队列失败(%s): %w", spec.QueueName, err)
	}
	return channel, deliveries, nil
}

// consumeQueue 顺序消费单个业务队列中的消息，并在处理失败时执行重试或死信转发。
func (c *Consumer) consumeQueue(
	ctx context.Context,
	channel *amqp.Channel,
	spec QueueSpec,
	handler TaskHandler,
	deliveries <-chan amqp.Delivery,
	sessionErrCh chan<- error,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-deliveries:
			if !ok {
				return
			}
			if err := c.handleDelivery(ctx, channel, spec, handler, delivery); err != nil {
				select {
				case sessionErrCh <- err:
				default:
				}
				return
			}
		}
	}
}

// handleDelivery 处理单条消息并维护确认、重试与死信流程。
func (c *Consumer) handleDelivery(
	ctx context.Context,
	channel *amqp.Channel,
	spec QueueSpec,
	handler TaskHandler,
	delivery amqp.Delivery,
) error {
	var message TaskMessage
	if err := json.Unmarshal(delivery.Body, &message); err != nil {
		applogger.Error("消息解析失败，转入死信队列",
			zap.String("queue", spec.QueueName),
			zap.Error(err),
		)
		if publishErr := publishToDeadLetter(ctx, channel, spec, delivery, fmt.Errorf("解析任务消息失败: %w", err)); publishErr != nil {
			return publishErr
		}
		return delivery.Ack(false)
	}

	applogger.Info("开始处理任务",
		zap.String("task_type", message.TaskType),
		zap.Uint("task_id", message.TaskID),
		zap.String("message_id", message.MessageID),
		zap.String("queue", spec.QueueName),
		zap.Int("attempt", message.Attempt),
	)
	start := time.Now()

	if err := handler.Handle(ctx, message); err != nil {
		elapsed := time.Since(start)
		retryCount := readRetryCount(delivery.Headers)
		if retryCount < spec.MaxRetries {
			applogger.Warn("任务处理失败，将重试",
				zap.String("task_type", message.TaskType),
				zap.Uint("task_id", message.TaskID),
				zap.Duration("elapsed", elapsed),
				zap.Int("retry_count", retryCount+1),
				zap.Int("max_retries", spec.MaxRetries),
				zap.Error(err),
			)
			if publishErr := publishToRetry(ctx, channel, spec, delivery, retryCount+1, err); publishErr != nil {
				return publishErr
			}
		} else {
			applogger.Error("任务处理失败且重试次数耗尽，转入死信队列",
				zap.String("task_type", message.TaskType),
				zap.Uint("task_id", message.TaskID),
				zap.Duration("elapsed", elapsed),
				zap.Int("retry_count", retryCount),
				zap.Error(err),
			)
			if publishErr := publishToDeadLetter(ctx, channel, spec, delivery, err); publishErr != nil {
				return publishErr
			}
		}
		return delivery.Ack(false)
	}

	applogger.Info("任务处理完成",
		zap.String("task_type", message.TaskType),
		zap.Uint("task_id", message.TaskID),
		zap.Duration("elapsed", time.Since(start)),
	)
	return delivery.Ack(false)
}

// publishToRetry 将失败消息投递到重试队列，并带上递增后的重试次数。
func publishToRetry(
	ctx context.Context,
	channel *amqp.Channel,
	spec QueueSpec,
	delivery amqp.Delivery,
	retryCount int,
	taskErr error,
) error {
	headers := cloneHeaders(delivery.Headers)
	headers["x-retry-count"] = retryCount
	headers["x-last-error"] = taskErr.Error()
	if err := channel.PublishWithContext(ctx, RetryExchangeName, spec.RoutingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         delivery.Body,
		Headers:      headers,
		Timestamp:    time.Now(),
	}); err != nil {
		return fmt.Errorf("投递重试消息失败(%s): %w", spec.QueueName, err)
	}
	return nil
}

// publishToDeadLetter 将超过重试次数或格式错误的消息转入死信队列。
func publishToDeadLetter(
	ctx context.Context,
	channel *amqp.Channel,
	spec QueueSpec,
	delivery amqp.Delivery,
	taskErr error,
) error {
	headers := cloneHeaders(delivery.Headers)
	headers["x-last-error"] = taskErr.Error()
	if err := channel.PublishWithContext(ctx, DeadLetterExchangeName, spec.RoutingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         delivery.Body,
		Headers:      headers,
		Timestamp:    time.Now(),
	}); err != nil {
		return fmt.Errorf("投递死信消息失败(%s): %w", spec.QueueName, err)
	}
	return nil
}

// readRetryCount 从 RabbitMQ header 中解析当前消息的重试次数。
func readRetryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	switch value := headers["x-retry-count"].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case uint8:
		return int(value)
	default:
		return 0
	}
}

// cloneHeaders 复制消息头，避免在原始 delivery 头上原地改写。
func cloneHeaders(headers amqp.Table) amqp.Table {
	if len(headers) == 0 {
		return amqp.Table{}
	}
	cloned := make(amqp.Table, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

// closeConsumerChannels 统一关闭本轮会话打开的所有消费通道。
func closeConsumerChannels(closers []func()) {
	for _, closer := range closers {
		closer()
	}
}
