package middleware

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc"
)

// Logging 返回 gRPC 日志拦截器
func Logging() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		reply, err := handler(ctx, req)
		duration := time.Since(start)

		level := log.LevelInfo
		if err != nil {
			level = log.LevelError
		}

		log.Context(ctx).Log(level,
			"method", info.FullMethod,
			"duration", duration.String(),
			"err", err,
		)

		return reply, err
	}
}
