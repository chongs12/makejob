package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// InitTracerProvider 初始化 OTel TracerProvider，通过 OTLP gRPC 导出到 Collector。
//
//   - serviceName 落入 resource 的 service.name，Jaeger 据此按服务分组
//   - otlpEndpoint 为 Collector 地址（如 "localhost:4317" 或 K8s 中 "collector:4317"）
//   - sampleRatio 为 TraceIDRatioBased 采样率（1.0 = 全量）；demo 用 AlwaysOn(1.0)
//
// 内部显式调用 otel.SetTracerProvider 与 otel.SetTextMapPropagator（TraceContext+Baggage），
// 否则 otelgrpc 会使用 noop provider，trace 静默丢失。
func InitTracerProvider(serviceName, otlpEndpoint string, sampleRatio float64) (*sdktrace.TracerProvider, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		// Collector 为明文 gRPC（本地 dev / 集群内），用 WithInsecure 跳过 TLS。
		// 注意：WithDialOption(grpc.WithTransportCredentials(insecure)) 不生效——
		// otlptracegrpc 默认 TLS，必须用 WithInsecure/WithTLSCredentials 覆盖。
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("telemetry: create OTLP trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("service.version", serviceVersion()),
			attribute.String("deployment.environment", envOrDefault()),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("telemetry: create trace resource: %w", err)
	}

	// ParentBased(TraceIDRatioBased) 确保子 span 遵循上游采样决策，
	// demo sampleRatio=1.0 时整条链路全量采样。
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	cleanup := func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := tp.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "telemetry: tracer provider shutdown: %v\n", err)
		}
	}
	return tp, cleanup, nil
}

// Tracer 返回具名 tracer，供业务层手动埋点使用（如 ark.chat / rag.retrieve）。
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// envOrDefault 返回部署环境标识，由 OTEL_ENV 环境变量控制，默认 development。
func envOrDefault() string {
	if env := os.Getenv("OTEL_ENV"); env != "" {
		return env
	}
	return "development"
}

// serviceVersion 返回服务版本，由 SERVICE_VERSION 环境变量控制，默认 v1.0.0。
func serviceVersion() string {
	if v := os.Getenv("SERVICE_VERSION"); v != "" {
		return v
	}
	return "v1.0.0"
}
