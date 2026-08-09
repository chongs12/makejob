package proxy

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// 业务 HTTP 指标（注册到 prometheus.DefaultRegisterer，与 :6060 telemetry 端点共用）。
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests processed by gateway.",
		},
		[]string{"method", "path", "code"},
	)
	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request handling duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDurationSeconds)
}

// HTTPMetricsMiddleware 记录 gateway 的 HTTP 请求指标。
//
// path 使用 gin 路由模板（c.FullPath()，如 /api/v1/interviews/:id）而非实际 URL，
// 避免高基数 label 导致 Prometheus 指标爆炸。
func HTTPMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}
		httpRequestsTotal.WithLabelValues(c.Request.Method, path, strconv.Itoa(c.Writer.Status())).Inc()
		httpRequestDurationSeconds.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}
