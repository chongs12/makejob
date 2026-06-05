package server

import (
	"makejob/app/coderunner/internal/conf"
	"makejob/app/coderunner/internal/service"
	"makejob/pkg/middleware"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"

	coderunnerv1 "makejob/api/makejob/coderunner/v1"
)

// NewGRPCServer 创建 Kratos gRPC 服务器（内部服务，不使用 JWT 认证）
func NewGRPCServer(
	cfg *conf.Server,
	coderunnerSvc *service.CodeRunnerService,
	logger log.Logger,
) *kratosgrpc.Server {
	opts := []kratosgrpc.ServerOption{
		kratosgrpc.Logger(logger),
		// 中间件链：Recovery + Logging（内部服务无需 JWT）
		kratosgrpc.UnaryInterceptor(
			middleware.Recovery(),
			middleware.Logging(),
		),
	}

	if cfg.GRPC != nil && cfg.GRPC.Addr != "" {
		opts = append(opts, kratosgrpc.Address(cfg.GRPC.Addr))
	}

	srv := kratosgrpc.NewServer(opts...)
	coderunnerv1.RegisterCodeRunnerServiceServer(srv.Server, coderunnerSvc)
	return srv
}
