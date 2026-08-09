package server

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc"

	"makejob/app/rag/internal/conf"
	"makejob/app/rag/internal/service"
	"makejob/pkg/server"

	ragv1 "makejob/api/makejob/rag/v1"
)

// NewGRPCServer 构造 rag 服务的 gRPC server。
// 拦截器链（otelgrpc -> prometheus -> recovery -> logging -> auth）由 pkg/server.NewGRPCServer 统一装配。
func NewGRPCServer(
	cfg *conf.Server,
	ragSvc *service.RAGService,

	logger log.Logger,
) *kratosgrpc.Server {
	var addr string
	var timeout time.Duration
	if cfg.GRPC != nil {
		addr = cfg.GRPC.Addr
		if cfg.GRPC.Timeout != "" {
			timeout, _ = time.ParseDuration(cfg.GRPC.Timeout)
		}
	}
	return server.NewGRPCServer(addr, timeout, nil, func(s *grpc.Server) {
		ragv1.RegisterRAGServiceServer(s, ragSvc)
	}, logger)
}
