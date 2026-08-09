package server

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc"

	"makejob/app/question/internal/conf"
	"makejob/app/question/internal/service"
	"makejob/pkg/auth"
	"makejob/pkg/server"

	questionv1 "makejob/api/makejob/question/v1"
)

// NewGRPCServer 构造 question 服务的 gRPC server。
// 拦截器链（otelgrpc -> prometheus -> recovery -> logging -> auth）由 pkg/server.NewGRPCServer 统一装配。
func NewGRPCServer(
	cfg *conf.Server,
	questionSvc *service.QuestionService,
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
		questionv1.RegisterQuestionServiceServer(s, questionSvc)
	}, logger)
}
