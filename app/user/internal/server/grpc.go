package server

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc"

	"makejob/app/user/internal/conf"
	"makejob/app/user/internal/service"
	"makejob/pkg/auth"
	"makejob/pkg/server"

	userv1 "makejob/api/makejob/user/v1"
)

// NewGRPCServer 构造 user 服务的 gRPC server。
// 拦截器链（otelgrpc -> prometheus -> recovery -> logging -> auth）由 pkg/server.NewGRPCServer 统一装配。
func NewGRPCServer(
	cfg *conf.Server,
	userSvc *service.UserService,
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
		userv1.RegisterUserServiceServer(s, userSvc)
	}, logger)
}
