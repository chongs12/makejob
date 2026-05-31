// Package middleware 提供Gin中间件功能
package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Tracing OpenTelemetry 分布式追踪中间件。
// 自动为每个 HTTP 请求创建 span，并将 traceID 注入 Gin context 供日志中间件使用。
// 需放在 RequestID 中间件之后、Logger 中间件之前。
func Tracing() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 HTTP header 中提取传播上下文（W3C TraceContext）
		ctx := otel.GetTextMapPropagator().Extract(
			c.Request.Context(),
			propagation.HeaderCarrier(c.Request.Header),
		)

		// 创建 span
		tracer := otel.Tracer("makejob-http")
		ctx, span := tracer.Start(ctx,
			fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path),
			trace.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.url", c.Request.URL.String()),
				attribute.String("http.client_ip", c.ClientIP()),
			),
		)
		defer span.End()

		// 将 span context 注入 request context
		c.Request = c.Request.WithContext(ctx)

		// 将 traceID 存入 Gin context，供日志中间件读取
		spanCtx := span.SpanContext()
		if spanCtx.HasTraceID() {
			c.Set("trace_id", spanCtx.TraceID().String())
		}

		c.Next()

		// 请求结束后记录状态码
		statusCode := c.Writer.Status()
		span.SetAttributes(attribute.Int("http.status", statusCode))
		if statusCode >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
		}
	}
}
