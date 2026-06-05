package server

import (
	"makejob/app/growth/internal/conf"
	"makejob/app/growth/internal/service"
	"makejob/pkg/auth"
	"makejob/pkg/middleware"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"

	growthv1 "makejob/api/makejob/growth/v1"
)

// NewGRPCServer 创建 Kratos gRPC 服务器
func NewGRPCServer(
	cfg *conf.Server,
	growthSvc *service.GrowthService,
	authInterceptor *auth.Interceptor,
	logger log.Logger,
) *kratosgrpc.Server {
	opts := []kratosgrpc.ServerOption{
		kratosgrpc.Logger(logger),
		// 统一中间件链
		kratosgrpc.UnaryInterceptor(
			middleware.Recovery(),                      // 1. panic 恢复
			middleware.Logging(),                       // 2. 请求日志
			authInterceptor.UnaryServerInterceptor(),  // 3. JWT 认证
		),
	}

	if cfg.GRPC != nil && cfg.GRPC.Addr != "" {
		opts = append(opts, kratosgrpc.Address(cfg.GRPC.Addr))
	}

	srv := kratosgrpc.NewServer(opts...)
	growthv1.RegisterGrowthServiceServer(srv.Server, growthSvc)
	return srv
}
