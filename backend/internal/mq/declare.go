// Package mq 提供 RabbitMQ 消息定义与基础设施封装。
package mq

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// declareTopology 声明当前系统使用的交换机、业务队列、重试队列和死信队列。
func declareTopology(channel *amqp.Channel, specs []QueueSpec) error {
	if err := channel.ExchangeDeclare(TasksExchangeName, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明任务交换机失败: %w", err)
	}
	if err := channel.ExchangeDeclare(RetryExchangeName, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明重试交换机失败: %w", err)
	}
	if err := channel.ExchangeDeclare(DeadLetterExchangeName, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明死信交换机失败: %w", err)
	}

	for _, spec := range specs {
		if err := declareBusinessQueue(channel, spec); err != nil {
			return err
		}
		if err := declareRetryQueue(channel, spec); err != nil {
			return err
		}
		if err := declareDeadLetterQueue(channel, spec); err != nil {
			return err
		}
	}
	return nil
}

// declareBusinessQueue 声明正式消费的业务队列并绑定主交换机。
func declareBusinessQueue(channel *amqp.Channel, spec QueueSpec) error {
	if _, err := channel.QueueDeclare(spec.QueueName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明业务队列失败(%s): %w", spec.QueueName, err)
	}
	if err := channel.QueueBind(spec.QueueName, spec.RoutingKey, TasksExchangeName, false, nil); err != nil {
		return fmt.Errorf("绑定业务队列失败(%s): %w", spec.QueueName, err)
	}
	return nil
}

// declareRetryQueue 声明带 TTL 的重试队列，并在超时后重新投回主交换机。
func declareRetryQueue(channel *amqp.Channel, spec QueueSpec) error {
	args := amqp.Table{
		"x-message-ttl":             int32(spec.RetryDelay / time.Millisecond),
		"x-dead-letter-exchange":    TasksExchangeName,
		"x-dead-letter-routing-key": spec.RoutingKey,
	}
	if _, err := channel.QueueDeclare(spec.RetryQueueName(), true, false, false, false, args); err != nil {
		return fmt.Errorf("声明重试队列失败(%s): %w", spec.RetryQueueName(), err)
	}
	if err := channel.QueueBind(spec.RetryQueueName(), spec.RoutingKey, RetryExchangeName, false, nil); err != nil {
		return fmt.Errorf("绑定重试队列失败(%s): %w", spec.RetryQueueName(), err)
	}
	return nil
}

// declareDeadLetterQueue 声明死信队列，方便后续排障与人工补偿。
func declareDeadLetterQueue(channel *amqp.Channel, spec QueueSpec) error {
	if _, err := channel.QueueDeclare(spec.DeadLetterQueueName(), true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明死信队列失败(%s): %w", spec.DeadLetterQueueName(), err)
	}
	if err := channel.QueueBind(spec.DeadLetterQueueName(), spec.RoutingKey, DeadLetterExchangeName, false, nil); err != nil {
		return fmt.Errorf("绑定死信队列失败(%s): %w", spec.DeadLetterQueueName(), err)
	}
	return nil
}
