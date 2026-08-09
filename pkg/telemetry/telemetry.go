package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Config 可观测性配置（对应各服务 config.yaml 的 telemetry 段）。
//
// 示例 yaml：
//
//	telemetry:
//	  otlp_endpoint: "localhost:4317"   # K8s 中为 collector:4317
//	  service_name: "makejob.interview"
//	  sample_ratio: 1.0                  # demo 用 1.0 全量采样
//	  http_port: 6060                    # 本地 dev 写 6060+服务序号，K8s ConfigMap 覆盖为 6060
type Config struct {
	OTLPEndpoint string  `yaml:"otlp_endpoint"`
	ServiceName  string  `yaml:"service_name"`
	SampleRatio  float64 `yaml:"sample_ratio"`
	HTTPPort     int     `yaml:"http_port"`
}

// defaultHTTPPort 为未配置 http_port 时的兜底端口。
const defaultHTTPPort = 6060

// Init 一站式初始化 tracer + meter + 可观测性 HTTP server，返回 cleanup。
//
// 调用方应在 main.go 中：
//
//	cleanup, err := telemetry.Init(cfg)
//	if err != nil { ... }
//	defer cleanup()
//
// 内部使用 prometheus.DefaultRegisterer，使 gRPC 指标（go-grpc-prometheus）、
// AI/HTTP 业务指标（client_golang）与 OTel 指标共用同一 registry，
// 统一通过 :6060/metrics 暴露给 Prometheus 抓取。
func Init(cfg Config) (cleanup func(), err error) {
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = defaultHTTPPort
	}
	if cfg.SampleRatio <= 0 {
		cfg.SampleRatio = 1.0 // demo 默认全量采样
	}

	_, tpCleanup, err := InitTracerProvider(cfg.ServiceName, cfg.OTLPEndpoint, cfg.SampleRatio)
	if err != nil {
		return nil, err
	}

	_, mpCleanup, err := InitMeterProvider(cfg.ServiceName)
	if err != nil {
		tpCleanup()
		return nil, err
	}

	srv := NewHTTPServer(cfg.HTTPPort, prometheus.DefaultGatherer)
	Serve(srv, func(e error) {
		fmt.Fprintf(os.Stderr, "telemetry: http server error: %v\n", e)
	})

	cleanup = func() {
		// 停机顺序：先停 HTTP（停止接收抓取/探针），再 flush trace/meter。
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		mpCleanup()
		tpCleanup()
	}
	return cleanup, nil
}
