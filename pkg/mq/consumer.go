package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
//
// trace 传播（对应决策 4 / 坑 4）：
//   - 每条消息从 delivery.Headers 提取 traceparent，创建 consumer span，派生 msgCtx
//   - handler 使用 msgCtx 而非共享的 Start(ctx)，使 trace 跨 MQ 链路连续（interview.finished -> learning_archive）
//   - 重试用 span event 记录（attempt/err），不创建新 span，避免重试产生碎片 trace
//   - 日志改用 log.Context(msgCtx)，使 MQ 日志带上 consumer span 的 trace_id（依赖 1.6 的 Valuer 机制）
func (c *Consumer) processMessages(ctx context.Context, queueName string, msgs <-chan amqp.Delivery, handler TaskHandler) {
	tracer := otel.Tracer("makejob.mq")
	propagator := otel.GetTextMapPropagator()

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

		// per-message: 从 AMQP Headers 提取 traceparent，创建 consumer span，派生 msgCtx。
		parentCtx := propagator.Extract(ctx, amqpHeaderCarrier(delivery.Headers))
		msgCtx, span := tracer.Start(parentCtx, "mq.consume."+msg.TaskType,
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("messaging.system", "rabbitmq"),
				attribute.String("messaging.destination", queueName),
				attribute.String("messaging.destination_kind", "queue"),
				attribute.String("messaging.operation", "process"),
				attribute.String("messaging.message_id", fmt.Sprintf("%s-%d", msg.EntityType, msg.EntityID)),
			),
		)

		var lastErr error
		for attempt := 0; attempt <= msg.RetryCount; attempt++ {
			log.Context(msgCtx).Infof("MQ handler start: queue=%s task=%s entity_id=%d attempt=%d/%d", queueName, msg.TaskType, msg.EntityID, attempt+1, msg.RetryCount+1)
			handleStart := time.Now()
			if err := handler.Handle(msgCtx, msg); err != nil {
				lastErr = err
				log.Context(msgCtx).Errorf("MQ handler failed: queue=%s task=%s entity_id=%d attempt=%d/%d duration=%dms err=%v", queueName, msg.TaskType, msg.EntityID, attempt+1, msg.RetryCount+1, time.Since(handleStart).Milliseconds(), err)
				// 重试用 span event 记录，不创建新 span
				span.AddEvent("retry", trace.WithAttributes(
					attribute.Int("attempt", attempt+1),
					attribute.String("error", err.Error()),
				))
				select {
				case <-c.done:
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					span.End()
					delivery.Nack(false, true)
					return
				case <-time.After(time.Duration(attempt+1) * time.Second):
				}
				continue
			}
			lastErr = nil
			log.Context(msgCtx).Infof("MQ handler done: queue=%s task=%s entity_id=%d duration=%dms", queueName, msg.TaskType, msg.EntityID, time.Since(handleStart).Milliseconds())
			break
		}

		if lastErr != nil {
			log.Context(msgCtx).Errorf("MQ handler final failure: queue=%s err=%v", queueName, lastErr)
			span.RecordError(lastErr)
			span.SetStatus(codes.Error, lastErr.Error())
			if failureHandler, ok := handler.(TaskFailureHandler); ok {
				_ = failureHandler.HandleFinalFailure(msgCtx, msg, lastErr)
			}
			delivery.Nack(false, false)
		} else {
			delivery.Ack(false)
		}
		span.End()
	}
}
