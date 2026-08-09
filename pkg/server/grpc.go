package server

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"

	"makejob/pkg/middleware"
)

// defaultTimeout 为未配置或非法 timeout 时的兜底值。
// 注意：现有 8 个服务（user/membership/learning_archive/rag/realtime/growth/community/coderunner）
// 原本无 timeout 逻辑、跑在 Kratos 1s 默认值下；统一构造后从 config 读取，learning_archive
// 未配 timeout 字段时走此默认值（对应风险 #8，阶段 2 需验收行为变更无异常）。
const defaultTimeout = 10 * time.Second

// NewGRPCServer 统一构造 Kratos gRPC Server，装配标准可观测性拦截器链，并注册 gRPC 指标。
//
// 拦截器链顺序（在 pkg/server 一处控制，14 个服务共享）：
//
//	otelgrpc -> prometheus -> recovery -> logging -> auth
//
//   - otelgrpc 最外层：确保 span 在 logging/recovery 求值 trace_id（log.Valuer）前已注入 ctx
//   - prometheus：go-grpc-prometheus 产出 grpc_server_handled_total 等标准指标
//   - auth：authInterceptor 为 nil 时省略（rag/coderunner 无 auth）
//
// 参数：
//   - addr: gRPC 监听地址（如 ":9004"）
//   - timeout: 单次 RPC 超时；<=0 时用 defaultTimeout 兜底
//   - authInterceptor: 服务端 auth 拦截器，nil 表示无 auth
//   - register: 回调，注册各服务的 proto service 实现
//   - logger: Kratos transport 日志器
//   - extraOpts: 标准 grpc.ServerOption，透传给底层 grpc.Server
//
// 各服务 auth 差异由调用方构造自己的 interceptor 传入：
//
//	auth.NewInterceptor(secret).UnaryServerInterceptor()                     // 普通服务
//	auth.NewInterceptor(secret, auth.WithBlacklistChecker(...)).UnaryServerInterceptor() // user 黑名单变体
//	nil                                                                     // rag/coderunner
//
// 不要用 extraOpts 注入 auth 拦截器（grpc.ServerOption 只会 append 到链尾，导致双重 auth）。
func NewGRPCServer(
	addr string,
	timeout time.Duration,
	authInterceptor grpc.UnaryServerInterceptor,
	register func(*grpc.Server),
	logger log.Logger,
	extraOpts ...grpc.ServerOption,
) *kratosgrpc.Server {
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	interceptors := []grpc.UnaryServerInterceptor{
		middleware.TracingServerInterceptor(), // otelgrpc，最外层
		middleware.MetricsServerInterceptor(), // go-grpc-prometheus
		middleware.Recovery(),
		middleware.Logging(),
	}
	if authInterceptor != nil {
		interceptors = append(interceptors, authInterceptor)
	}

	opts := []kratosgrpc.ServerOption{
		kratosgrpc.Logger(logger),
		kratosgrpc.UnaryInterceptor(interceptors...),
		kratosgrpc.Timeout(timeout),
	}
	if addr != "" {
		opts = append(opts, kratosgrpc.Address(addr))
	}
	if len(extraOpts) > 0 {
		opts = append(opts, kratosgrpc.Options(extraOpts...))
	}

	srv := kratosgrpc.NewServer(opts...)

	// 先注册业务 service，再 Register metrics（go-grpc-prometheus 标准顺序：
	// InitializeMetrics 遍历已注册 service 为每个 method 预创建计数器）。
	if register != nil {
		register(srv.Server)
	}
	grpc_prometheus.Register(srv.Server)

	return srv
}
