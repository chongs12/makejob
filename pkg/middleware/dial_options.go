package middleware

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// CommonDialOptions 返回统一的 gRPC 客户端 Dial 选项基座（不含 auth）。
//
// auth 拦截器由各客户端按场景追加（grpc-go 的 WithUnaryInterceptor 是赋值覆盖，
// 多个拦截器必须用 WithChainUnaryInterceptor 追加，故基座用 chain、auth 用 append single）：
//
//	// 无 auth（interview -> ai/rag/code_runner）
//	conn, _ := grpc.Dial(addr, middleware.CommonDialOptions()...)
//
//	// ServiceAuth（interview -> archive/membership/companion）
//	opts := append(middleware.CommonDialOptions(),
//	    grpc.WithUnaryInterceptor(auth.ServiceAuthInterceptor(token)))
//	conn, _ := grpc.Dial(addr, opts...)
//
//	// ForwardToken（companion -> interview，gateway 转发用户 JWT）
//	opts := append(middleware.CommonDialOptions(),
//	    grpc.WithUnaryInterceptor(auth.ForwardTokenClientInterceptor()))
//	conn, _ := grpc.Dial(addr, opts...)
//
// 最终拦截器执行顺序：otelgrpc(client) -> [auth] -> invoker，tracing 覆盖整个调用。
func CommonDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			TracingClientInterceptor(),
			// client 侧 metrics 为可选项；当前阶段未启用，如需可追加
			// grpc_prometheus 的 client interceptor 并 EnableClientHandlingTimeHistogram。
		),
	}
}
