package middleware

import (
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
)

// MetricsServerInterceptor 返回 gRPC 服务端指标拦截器。
//
// 使用 go-grpc-prometheus（go.mod 已有 indirect v1.2.0）的默认 ServerMetrics，
// 产出标准指标名：
//   - grpc_server_started_total
//   - grpc_server_handled_total{grpc_method,grpc_code}
//   - grpc_server_handling_seconds_bucket（需 EnableHandlingTimeHistogram）
//
// 注意：不要自定义 grpc_requests_total / grpc_request_duration_seconds，会与
// go-grpc-prometheus 标准名对不上，导致 Grafana 面板空转。
//
// ServerMetrics 在包 init() 时已注册到 prometheus.DefaultRegisterer，
// pkg/server.NewGRPCServer 内部还会调用 grpc_prometheus.Register(srv) 初始化方法级计数。
func MetricsServerInterceptor() grpc.UnaryServerInterceptor {
	return grpc_prometheus.UnaryServerInterceptor
}
