package biz

import (
	"github.com/prometheus/client_golang/prometheus"
)

// AI 调用业务指标（注册到 prometheus.DefaultRegisterer，通过 :6060/metrics 暴露）。
// 在各 usecase 的 saveLog 内统一打点（scene/model/status/tokens/latency 集中于此）。
var (
	aiCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_calls_total",
			Help: "Total number of AI (LLM) calls grouped by scene/model/status.",
		},
		[]string{"scene", "model", "status"},
	)
	aiTokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_tokens_total",
			Help: "Total tokens consumed by AI calls (prompt/completion).",
		},
		[]string{"scene", "model", "type"}, // type = prompt | completion
	)
	aiCallDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_call_duration_seconds",
			Help:    "AI call duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"scene", "model"},
	)
)

// RegisterAIMetrics 显式注册 AI 指标到 prometheus.DefaultRegisterer。
//
// 在 main.go 的 telemetry.Init 之后调用（而非 init()）：显式注册确保注册时机明确，
// 且与 go-grpc-prometheus 的注册（pkg/server.NewGRPCServer 内 grpc_prometheus.Register）
// 一致地落到 :6060 实际采集的 prometheus.DefaultRegisterer。
//
// 注意：CounterVec/HistogramVec 在无 WithLabelValues 调用时不输出 MetricFamily
// （prometheus 行为），故未发生 AI 调用时 :6060/metrics 看不到 ai_ 指标，属正常。
func RegisterAIMetrics() {
	prometheus.MustRegister(aiCallsTotal, aiTokensTotal, aiCallDurationSeconds)
}

// recordAICallMetrics 在 saveLog 内调用，记录单次 AI 调用的指标。
// 覆盖所有 usecase（InterviewAgent/PlanAgent/CompanionAgent/QuizAnalyzer/ResumeParser/Live2DDirector/Admin/InterviewSession）。
func recordAICallMetrics(scene, model, status string, promptTokens, completionTokens int, latencyMs int64) {
	aiCallsTotal.WithLabelValues(scene, model, status).Inc()
	if status == "success" {
		aiTokensTotal.WithLabelValues(scene, model, "prompt").Add(float64(promptTokens))
		aiTokensTotal.WithLabelValues(scene, model, "completion").Add(float64(completionTokens))
	}
	aiCallDurationSeconds.WithLabelValues(scene, model).Observe(float64(latencyMs) / 1000)
}
