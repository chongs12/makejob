package server

import (
	archivev1 "makejob/api/makejob/learning_archive/v1"
	"makejob/app/learning_archive/internal/conf"
	"makejob/app/learning_archive/internal/service"
	"makejob/pkg/auth"
	"makejob/pkg/middleware"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
)

func NewGRPCServer(
	cfg *conf.Server,
	archiveSvc *service.ArchiveService,
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
	archivev1.RegisterLearningArchiveServiceServer(srv.Server, archiveSvc)
	return srv
}
