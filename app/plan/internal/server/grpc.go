package server

import (
	"time"

	"makejob/app/plan/internal/conf"
	"makejob/app/plan/internal/service"
	"makejob/pkg/auth"
	"makejob/pkg/middleware"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"

	planv1 "makejob/api/makejob/plan/v1"
)

// NewGRPCServer 创建 Kratos gRPC 服务器
func NewGRPCServer(
	cfg *conf.Server,
	planSvc *service.PlanService,
	authInterceptor *auth.Interceptor,
	logger log.Logger,
) *kratosgrpc.Server {
	opts := []kratosgrpc.ServerOption{
		kratosgrpc.Logger(logger),
		// 统一中间件链
		kratosgrpc.UnaryInterceptor(
			middleware.Recovery(),                    // 1. panic 恢复
			middleware.Logging(),                     // 2. 请求日志
			authInterceptor.UnaryServerInterceptor(), // 3. JWT 认证
		),
	}

	if cfg.GRPC != nil && cfg.GRPC.Addr != "" {
		opts = append(opts, kratosgrpc.Address(cfg.GRPC.Addr))
	}

	// 设置 gRPC 超时，默认 3 分钟（学习计划生成依赖 AI Gateway LLM 调用，耗时较长）
	if cfg.GRPC != nil && cfg.GRPC.Timeout != "" {
		if d, err := time.ParseDuration(cfg.GRPC.Timeout); err == nil {
			opts = append(opts, kratosgrpc.Timeout(d))
		}
	} else {
		opts = append(opts, kratosgrpc.Timeout(3*time.Minute))
	}

	srv := kratosgrpc.NewServer(opts...)
	planv1.RegisterPlanServiceServer(srv.Server, planSvc)
	return srv
}
