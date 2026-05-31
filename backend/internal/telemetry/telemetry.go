// Package telemetry 提供 OpenTelemetry 初始化与管理能力。
package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"makejob-backend/internal/config"
)

// Init 初始化 OpenTelemetry TracerProvider。
// 当 cfg.Enabled 为 false 时直接返回 noop cleanup，不创建任何 exporter。
// 返回 cleanup 函数，应在应用退出时调用以优雅关闭 TracerProvider。
func Init(ctx context.Context, cfg config.TelemetryConfig) (func(), error) {
	if !cfg.Enabled {
		return func() {}, nil
	}

	// 验证采样率范围
	sampleRate := cfg.SampleRate
	if sampleRate < 0 {
		sampleRate = 0
	} else if sampleRate > 1 {
		sampleRate = 1
	}

	// 创建 OTLP gRPC exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return func() {}, err
	}

	// 构建资源信息
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return func() {}, err
	}

	// 创建 TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(sampleRate)),
	)

	// 设置全局 TracerProvider
	otel.SetTracerProvider(tp)

	// 设置全局 Propagator（W3C TraceContext + Baggage）
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 返回 cleanup 函数
	return func() {
		_ = tp.Shutdown(ctx)
	}, nil
}

// GetTracer 获取命名的 Tracer。
// 当 TracerProvider 未初始化时返回 noop tracer。
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
