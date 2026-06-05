package server

import (
	"makejob/app/question/internal/conf"
	"makejob/app/question/internal/service"
	"makejob/pkg/auth"
	"makejob/pkg/middleware"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"

	questionv1 "makejob/api/makejob/question/v1"
)

func NewGRPCServer(
	cfg *conf.Server,
	questionSvc *service.QuestionService,
	authInterceptor *auth.Interceptor,
	logger log.Logger,
) *kratosgrpc.Server {
	opts := []kratosgrpc.ServerOption{
		kratosgrpc.Logger(logger),
		kratosgrpc.UnaryInterceptor(
			middleware.Recovery(),
			middleware.Logging(),
			authInterceptor.UnaryServerInterceptor(),
		),
	}

	if cfg.GRPC != nil && cfg.GRPC.Addr != "" {
		opts = append(opts, kratosgrpc.Address(cfg.GRPC.Addr))
	}

	srv := kratosgrpc.NewServer(opts...)
	questionv1.RegisterQuestionServiceServer(srv.Server, questionSvc)
	return srv
}
