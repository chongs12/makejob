package middleware

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

// TracingServerInterceptor 返回 gRPC 服务端 tracing 拦截器（封装 otelgrpc）。
//
// 必须位于拦截器链最外层（otelgrpc -> prometheus -> recovery -> logging -> auth），
// 以确保 span 在 Logging/Recovery 求值 trace_id（log.Valuer）前已注入 ctx。
//
// 前提：main.go 已调用 telemetry.Init 完成 otel.SetTracerProvider / SetTextMapPropagator，
// 否则 otelgrpc 使用 noop provider，trace 静默丢失。
func TracingServerInterceptor() grpc.UnaryServerInterceptor {
	return otelgrpc.UnaryServerInterceptor()
}

// TracingClientInterceptor 返回 gRPC 客户端 tracing 拦截器（封装 otelgrpc），
// 用于在跨服务 gRPC 调用时注入 traceparent 到 metadata，传播 trace 上下文。
//
// 由 pkg/middleware.CommonDialOptions() 统一装配到所有 gRPC 客户端。
func TracingClientInterceptor() grpc.UnaryClientInterceptor {
	return otelgrpc.UnaryClientInterceptor()
}
