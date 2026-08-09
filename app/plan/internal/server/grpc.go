package server

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc"

	"makejob/app/plan/internal/conf"
	"makejob/app/plan/internal/service"
	"makejob/pkg/auth"
	"makejob/pkg/server"

	planv1 "makejob/api/makejob/plan/v1"
)

// NewGRPCServer 构造 plan 服务的 gRPC server。
// 拦截器链（otelgrpc -> prometheus -> recovery -> logging -> auth）由 pkg/server.NewGRPCServer 统一装配。
func NewGRPCServer(
	cfg *conf.Server,
	planSvc *service.PlanService,
	authInterceptor *auth.Interceptor,
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
	return server.NewGRPCServer(addr, timeout, authInterceptor.UnaryServerInterceptor(), func(s *grpc.Server) {
		planv1.RegisterPlanServiceServer(s, planSvc)
	}, logger)
}
