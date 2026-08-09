package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewHTTPServer 创建独立的可观测性 HTTP server（默认 :6060），挂载：
//   - /metrics      Prometheus 抓取端点（采集 prometheus.DefaultGatherer）
//   - /healthz      liveness 探针（恒 200）
//   - /readyz       readiness 探针（可扩展检查逻辑）
//   - /debug/pprof/ Go pprof（不挂业务 server，独立端口暴露）
//
// 该 server 与业务 gRPC/HTTP server 完全解耦，单独 goroutine 启动。
//
// 注意：此处只需 Gatherer（采集/暴露），不需要 Registerer。
// go-grpc-prometheus 在 init() 时已 MustRegister 到 prometheus.DefaultRegisterer，
// client_golang 业务指标也注册到默认 registry，故 /metrics 用 DefaultGatherer 即可统一暴露。
func NewHTTPServer(port int, gatherer prometheus.Gatherer) *http.Server {
	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// readiness 可按服务扩展：检查下游依赖连通性等。当前默认就绪。
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// pprof 端点。独立 HTTP server 暴露，不污染业务端口；K8s 中不对外，仅 port-forward/ClusterIP 访问。
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
}

// Serve 在独立 goroutine 中启动可观测性 HTTP server，返回的 cleanup 用于优雅停机。
// 调用方应在 main.go 中：go func(){ ... }() 或直接调用（内部已开 goroutine）。
func Serve(srv *http.Server, onError func(error)) chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			if onError != nil {
				onError(err)
			}
		}
	}()
	return done
}

// Shutdown 优雅停机：等待 in-flight 请求完成或超时。
func Shutdown(srv *http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return srv.Shutdown(ctx)
}
