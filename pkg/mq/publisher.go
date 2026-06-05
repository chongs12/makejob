package mq

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher RabbitMQ 发布者（支持 publisher confirms）
type Publisher struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	exchange string
	confirms chan amqp.Confirmation
}

// NewPublisher 创建发布者
func NewPublisher(url, exchange string) (*Publisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// 启用 publisher confirms
	if err := ch.Confirm(false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to enable confirms: %w", err)
	}

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	return &Publisher{
		conn:     conn,
		channel:  ch,
		exchange: exchange,
		confirms: confirms,
	}, nil
}

// Publish 发布消息（带 publisher confirms）
func (p *Publisher) Publish(ctx context.Context, routingKey string, msg TaskMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := p.channel.PublishWithContext(ctx,
		p.exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			MessageId:   fmt.Sprintf("%s-%d", msg.EntityType, msg.EntityID),
		},
	); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	// 等待 broker 确认
	select {
	case confirm := <-p.confirms:
		if !confirm.Ack {
			return fmt.Errorf("message nacked by broker")
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// Close 关闭连接
func (p *Publisher) Close() error {
	if err := p.channel.Close(); err != nil {
		return err
	}
	return p.conn.Close()
}
