// Package metrics 提供 Prometheus 指标注册与采集能力。
package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Registry 是项目的 Prometheus 注册表，所有自定义指标在此注册。
	Registry = prometheus.NewRegistry()

	// HTTPRequestsTotal 记录 HTTP 请求总数。
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration 记录 HTTP 请求延迟分布。
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	// AICallsTotal 记录 AI 调用总数。
	AICallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_calls_total",
			Help: "Total number of AI model calls",
		},
		[]string{"scene", "provider", "model", "status"},
	)

	// AICallDuration 记录 AI 调用延迟分布。
	AICallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_call_duration_seconds",
			Help:    "AI model call latency in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"scene", "provider", "model"},
	)

	// AITokensTotal 记录 AI token 消耗总量。
	AITokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_tokens_total",
			Help: "Total AI tokens consumed",
		},
		[]string{"scene", "direction"},
	)

	// ActiveWebSocketConnections 记录当前活跃 WebSocket 连接数。
	ActiveWebSocketConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_websocket_connections",
			Help: "Number of active WebSocket connections",
		},
	)
)

func init() {
	Registry.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		AICallsTotal,
		AICallDuration,
		AITokensTotal,
		ActiveWebSocketConnections,
		// Go runtime 指标
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

// Handler 返回 Prometheus HTTP scrape 端点。
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{})
}

// NormalizePath 将动态路径段归一化，避免基数爆炸。
// 例如 /api/interview/123 → /api/interview/:id
func NormalizePath(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		// 纯数字段归一化为 :id
		if _, err := strconv.Atoi(part); err == nil {
			parts[i] = ":id"
			continue
		}
		// UUID 段归一化为 :id
		if len(part) == 36 && strings.Count(part, "-") == 4 {
			parts[i] = ":id"
		}
	}
	return "/" + strings.Join(parts, "/")
}

// RecordHTTPRequest 记录一次 HTTP 请求的指标。
func RecordHTTPRequest(method, path string, status int, duration time.Duration) {
	normalizedPath := NormalizePath(path)
	statusStr := strconv.Itoa(status)
	HTTPRequestsTotal.WithLabelValues(method, normalizedPath, statusStr).Inc()
	HTTPRequestDuration.WithLabelValues(method, normalizedPath).Observe(duration.Seconds())
}

// RecordAICall 记录一次 AI 调用的指标。
func RecordAICall(scene, provider, model string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}
	AICallsTotal.WithLabelValues(scene, provider, model, status).Inc()
	AICallDuration.WithLabelValues(scene, provider, model).Observe(duration.Seconds())
}

// RecordTokenUsage 记录 AI token 消耗。
func RecordTokenUsage(scene string, inputTokens, outputTokens int) {
	if inputTokens > 0 {
		AITokensTotal.WithLabelValues(scene, "input").Add(float64(inputTokens))
	}
	if outputTokens > 0 {
		AITokensTotal.WithLabelValues(scene, "output").Add(float64(outputTokens))
	}
}

// IncWebSocket 连接数 +1。
func IncWebSocket() {
	ActiveWebSocketConnections.Inc()
}

// DecWebSocket 连接数 -1。
func DecWebSocket() {
	ActiveWebSocketConnections.Dec()
}

// GinMetricsMiddleware 返回 Gin 中间件，自动记录 HTTP 请求指标。
func GinMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		RecordHTTPRequest(c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)
	}
}
