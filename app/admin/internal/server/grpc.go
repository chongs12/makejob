package server

import (
	"makejob/app/admin/internal/conf"
	"makejob/app/admin/internal/service"
	"makejob/pkg/auth"
	"makejob/pkg/middleware"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"

	adminv1 "makejob/api/makejob/admin/v1"
)

// NewGRPCServer 创建 Kratos gRPC 服务器
func NewGRPCServer(
	cfg *conf.Server,
	adminSvc *service.AdminService,
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
	adminv1.RegisterAdminServiceServer(srv.Server, adminSvc)
	return srv
}
