package mq

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	amqp "github.com/rabbitmq/amqp091-go"
)

func TestAMQPHeaderCarrierGetSet(t *testing.T) {
	c := amqpHeaderCarrier{}
	c.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")

	if got := c.Get("traceparent"); got != "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01" {
		t.Errorf("Get traceparent = %q, want injected value", got)
	}
	if got := c.Get("missing"); got != "" {
		t.Errorf("Get missing key = %q, want empty", got)
	}

	// 非 string 值应返回空（Get 只认 string）
	c["count"] = int64(42)
	if got := c.Get("count"); got != "" {
		t.Errorf("Get non-string value = %q, want empty", got)
	}
}

func TestAMQPHeaderCarrierKeys(t *testing.T) {
	c := amqpHeaderCarrier{"a": "1", "b": "2", "c": "3"}
	keys := c.Keys()
	if len(keys) != 3 {
		t.Fatalf("Keys len = %d, want 3", len(keys))
	}
}

// TestTracePropagationRoundTrip 验证 MQ trace 传递机制：
// publisher Inject traceparent 到 AMQP Headers -> consumer Extract 恢复 span context，
// trace_id 必须一致（不依赖真实 RabbitMQ broker）。
func TestTracePropagationRoundTrip(t *testing.T) {
	// 使用 W3C TraceContext propagator（与 telemetry.Init 设的全局一致）
	otel.SetTextMapPropagator(propagation.TraceContext{})

	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test")

	// 模拟 publisher：在 span 上下文中 Inject 到 AMQP Headers
	ctx, span := tracer.Start(context.Background(), "publish")
	defer span.End()

	headers := amqp.Table{}
	otel.GetTextMapPropagator().Inject(ctx, amqpHeaderCarrier(headers))

	if _, ok := headers["traceparent"]; !ok {
		t.Fatal("traceparent not injected into AMQP headers")
	}

	// 模拟 consumer：从 AMQP Headers Extract 恢复 trace 上下文（作为 remote parent）
	extractedCtx := otel.GetTextMapPropagator().Extract(context.Background(), amqpHeaderCarrier(headers))
	parentSC := trace.SpanContextFromContext(extractedCtx)
	if !parentSC.IsValid() {
		t.Fatal("extracted span context is invalid")
	}

	origSC := trace.SpanContextFromContext(ctx)
	if parentSC.TraceID() != origSC.TraceID() {
		t.Fatalf("trace_id mismatch: publisher=%s consumer_parent=%s",
			origSC.TraceID(), parentSC.TraceID())
	}

	// consumer 在 remote parent 之下创建新 span，应继承同一 trace_id 且拥有独立 SpanID
	consumerCtx, consumerSpan := tracer.Start(extractedCtx, "consume")
	defer consumerSpan.End()
	consumerSC := trace.SpanContextFromContext(consumerCtx)
	if consumerSC.TraceID() != origSC.TraceID() {
		t.Fatalf("consumer span trace_id mismatch: %s vs %s", consumerSC.TraceID(), origSC.TraceID())
	}
	if consumerSC.SpanID() == origSC.SpanID() {
		t.Fatal("consumer span 应有独立 SpanID，不应与 publisher span 相同")
	}
}
