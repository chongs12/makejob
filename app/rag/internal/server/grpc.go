package server

import (
	"makejob/app/rag/internal/conf"
	"makejob/app/rag/internal/service"
	"makejob/pkg/middleware"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"

	ragv1 "makejob/api/makejob/rag/v1"
)

// NewGRPCServer 创建 Kratos gRPC 服务器（内部服务，无 JWT 认证）
func NewGRPCServer(
	cfg *conf.Server,
	ragSvc *service.RAGService,
	logger log.Logger,
) *kratosgrpc.Server {
	opts := []kratosgrpc.ServerOption{
		kratosgrpc.Logger(logger),
		// 中间件链：panic 恢复 + 请求日志
		kratosgrpc.UnaryInterceptor(
			middleware.Recovery(),
			middleware.Logging(),
		),
	}

	if cfg.GRPC != nil && cfg.GRPC.Addr != "" {
		opts = append(opts, kratosgrpc.Address(cfg.GRPC.Addr))
	}

	srv := kratosgrpc.NewServer(opts...)
	ragv1.RegisterRAGServiceServer(srv.Server, ragSvc)
	return srv
}
