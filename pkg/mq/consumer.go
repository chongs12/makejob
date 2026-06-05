package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// TaskHandler 任务处理函数
type TaskHandler interface {
	Handle(ctx context.Context, msg TaskMessage) error
}

// TaskHandlerFunc 函数适配器
type TaskHandlerFunc func(ctx context.Context, msg TaskMessage) error

func (f TaskHandlerFunc) Handle(ctx context.Context, msg TaskMessage) error {
	return f(ctx, msg)
}

// Consumer RabbitMQ 消费者
type Consumer struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	handlers map[string]TaskHandler
	mu       sync.RWMutex
	done     chan struct{}
	once     sync.Once    // 防止 Stop 重复关闭 done channel
	wg       sync.WaitGroup // 等待所有 processMessages goroutine 退出
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

// Start 启动消费（实现 kratos/transport.Server 接口）
func (c *Consumer) Start(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for queueName, handler := range c.handlers {
		msgs, err := c.channel.Consume(
			queueName,
			"",    // consumer tag
			false, // auto-ack
			false, // exclusive
			false, // no-local
			false, // no-wait
			nil,   // args
		)
		if err != nil {
			return fmt.Errorf("failed to consume from %s: %w", queueName, err)
		}

		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.processMessages(ctx, queueName, msgs, handler)
		}()
	}

	// 阻塞直到 context 取消或 done 信号
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return nil
	}
}

// Stop 停止消费（实现 kratos/transport.Server 接口）
func (c *Consumer) Stop(ctx context.Context) error {
	c.once.Do(func() {
		close(c.done)
	})
	// 等待所有 processMessages goroutine 退出
	c.wg.Wait()
	if err := c.channel.Close(); err != nil {
		return err
	}
	return c.conn.Close()
}

func (c *Consumer) processMessages(ctx context.Context, queueName string, msgs <-chan amqp.Delivery, handler TaskHandler) {
	for delivery := range msgs {
		select {
		case <-c.done:
			// 收到停止信号，不再处理新消息
			delivery.Nack(false, true) // requeue
			return
		default:
		}

		var msg TaskMessage
		if err := json.Unmarshal(delivery.Body, &msg); err != nil {
			delivery.Nack(false, false)
			continue
		}

		// 带重试的处理（可中断）
		var lastErr error
		for attempt := 0; attempt <= msg.RetryCount; attempt++ {
			if err := handler.Handle(ctx, msg); err != nil {
				lastErr = err
				// 可中断的 sleep：监听 done channel 和超时
				select {
				case <-c.done:
					delivery.Nack(false, true) // requeue
					return
				case <-time.After(time.Duration(attempt+1) * time.Second):
					// 继续重试
				}
				continue
			}
			lastErr = nil
			break
		}

		if lastErr != nil {
			delivery.Nack(false, false) // 发送到死信队列
		} else {
			delivery.Ack(false)
		}
	}
}
