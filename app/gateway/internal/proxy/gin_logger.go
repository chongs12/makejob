package proxy

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-kratos/kratos/v2/log"
)

// GinLoggerMiddleware 替代 gin.Default() 自带的 Logger 中间件，
// 将 access log 走 kratos log（log.Context(ctx)），带上 otelgin span 的 trace_id/span_id。
//
// 必须在 otelgin.Middleware 之后注册，使 c.Request.Context() 已含 span。
// 这样每个请求的 access log 都关联 trace_id，可通过 trace_id 串联全链路日志。
func GinLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		ctx := c.Request.Context() // otelgin 已注入 span
		log.Context(ctx).Infow(
			"msg", "gin access",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", latency.Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}
