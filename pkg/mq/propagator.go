package mq

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

// amqpHeaderCarrier 将 amqp.Table 适配为 OTel propagation.TextMapCarrier，
// 使 W3C traceparent 可通过 AMQP 消息头注入/提取，不修改 TaskMessage 的 JSON schema。
//
// amqp.Table 即 map[string]interface{}，traceparent 头值为 string，可安全往返。
type amqpHeaderCarrier amqp.Table

// Get 读取 carrier 中的字符串值。
func (c amqpHeaderCarrier) Get(key string) string {
	if v, ok := c[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Set 写入字符串值（propagator.Inject 仅写 traceparent/baggage 头，均为 string）。
func (c amqpHeaderCarrier) Set(key, value string) {
	c[key] = value
}

// Keys 返回所有 key（propagator.Extract/Inject 在部分版本会调用）。
func (c amqpHeaderCarrier) Keys() []string {
	out := make([]string, 0, len(c))
	for k := range c {
		out = append(out, k)
	}
	return out
}
