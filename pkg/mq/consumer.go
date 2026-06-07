package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const defaultExchangeName = "makejob.async"

// TaskHandler 任务处理函数
type TaskHandler interface {
	Handle(ctx context.Context, msg TaskMessage) error
}

// TaskFailureHandler 在消息重试耗尽后处理最终失败状态。
type TaskFailureHandler interface {
	HandleFinalFailure(ctx context.Context, msg TaskMessage, lastErr error) error
}

// TaskHandlerFunc 函数适配器
type TaskHandlerFunc func(ctx context.Context, msg TaskMessage) error

// Handle 执行函数适配器包装的方法体。
func (f TaskHandlerFunc) Handle(ctx context.Context, msg TaskMessage) error {
	return f(ctx, msg)
}

// Consumer RabbitMQ 消费者
type Consumer struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	exchange string
	handlers map[string]TaskHandler
	mu       sync.RWMutex
	done     chan struct{}
	once     sync.Once
	wg       sync.WaitGroup
}

// NewConsumer 创建消费者
func NewConsumer(url string) (*Consumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	return &Consumer{
		conn:     conn,
		channel:  ch,
		exchange: defaultExchangeName,
		handlers: make(map[string]TaskHandler),
		done:     make(chan struct{}),
	}, nil
}

// Register 注册队列处理器
func (c *Consumer) Register(queueName string, handler TaskHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[queueName] = handler
}

// Start 启动消费并确保队列与绑定存在。
func (c *Consumer) Start(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.channel.ExchangeDeclare(c.exchange, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", c.exchange, err)
	}

	for queueName, handler := range c.handlers {
		if err := c.ensureQueueBinding(queueName); err != nil {
			return err
		}
		msgs, err := c.channel.Consume(
			queueName,
			"",
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to consume from %s: %w", queueName, err)
		}

		c.wg.Add(1)
		go func(localQueue string, localMsgs <-chan amqp.Delivery, localHandler TaskHandler) {
			defer c.wg.Done()
			c.processMessages(ctx, localQueue, localMsgs, localHandler)
		}(queueName, msgs, handler)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return nil
	}
}

// Stop 停止消费
func (c *Consumer) Stop(ctx context.Context) error {
	c.once.Do(func() {
		close(c.done)
	})
	c.wg.Wait()
	if err := c.channel.Close(); err != nil {
		return err
	}
	return c.conn.Close()
}

// ensureQueueBinding 声明队列并绑定到默认交换机。
func (c *Consumer) ensureQueueBinding(queueName string) error {
	if _, err := c.channel.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
	}
	routingKey := routingKeyForQueue(queueName)
	if routingKey == "" {
		return nil
	}
	if err := c.channel.QueueBind(queueName, routingKey, c.exchange, false, nil); err != nil {
		return fmt.Errorf("failed to bind queue %s with routing key %s: %w", queueName, routingKey, err)
	}
	return nil
}

// routingKeyForQueue 返回队列对应的路由键。
func routingKeyForQueue(queueName string) string {
	switch queueName {
	case QueuePlanGenerate:
		return RoutingKeyPlanGenerate
	case QueuePlanFeedbackDiagnosis:
		return RoutingKeyPlanFeedbackDiagnosis
	case QueueInterviewResumeParse:
		return RoutingKeyInterviewResumeParse
	case QueueInterviewReportGenerate:
		return RoutingKeyInterviewReportGenerate
	case QueueInterviewArchivePersist:
		return TaskTypeInterviewArchivePersist
	case QueueScraperImport:
		return TaskTypeScraperImport
	case QueueAdminQuestionPipeline:
		return TaskTypeAdminQuestionPipeline
	case QueueRAGSyncQuestion:
		return TaskTypeRAGSyncQuestion
	case QueueLearningArchiveInterviewFinished:
		return RoutingKeyInterviewFinished
	default:
		return ""
	}
}

// processMessages 处理消息并在最终失败时触发业务补偿。
func (c *Consumer) processMessages(ctx context.Context, queueName string, msgs <-chan amqp.Delivery, handler TaskHandler) {
	for delivery := range msgs {
		select {
		case <-c.done:
			delivery.Nack(false, true)
			return
		default:
		}

		var msg TaskMessage
		if err := json.Unmarshal(delivery.Body, &msg); err != nil {
			delivery.Nack(false, false)
			continue
		}

		var lastErr error
		for attempt := 0; attempt <= msg.RetryCount; attempt++ {
			if err := handler.Handle(ctx, msg); err != nil {
				lastErr = err
				select {
				case <-c.done:
					delivery.Nack(false, true)
					return
				case <-time.After(time.Duration(attempt+1) * time.Second):
				}
				continue
			}
			lastErr = nil
			break
		}

		if lastErr != nil {
			if failureHandler, ok := handler.(TaskFailureHandler); ok {
				_ = failureHandler.HandleFinalFailure(ctx, msg, lastErr)
			}
			delivery.Nack(false, false)
		} else {
			delivery.Ack(false)
		}
	}
}
