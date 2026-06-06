package auth

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// BlacklistChecker token 黑名单检查接口
type BlacklistChecker interface {
	IsBlacklisted(ctx context.Context, jti string) (bool, error)
}

// Interceptor JWT 认证拦截器
type Interceptor struct {
	secret          string
	publicMethods   map[string]bool
	blacklist       BlacklistChecker // FIX B1: 注入黑名单检查器
	logger          *log.Helper
}

// NewInterceptor 创建认证拦截器
func NewInterceptor(secret string, opts ...Option) *Interceptor {
	i := &Interceptor{
		secret: secret,
		publicMethods: map[string]bool{
			// User 服务公开方法
			"/makejob.user.v1.UserService/Register":     true,
			"/makejob.user.v1.UserService/Login":        true,
			"/makejob.user.v1.UserService/RefreshToken": true,
			// Question 服务公开方法
			"/makejob.question.v1.QuestionService/ListQuestions":    true,
			"/makejob.question.v1.QuestionService/GetQuestion":      true,
			"/makejob.question.v1.QuestionService/ListCategories":   true,
			"/makejob.question.v1.QuestionService/ListIndustries":   true,
			// Community 服务公开方法
			"/makejob.community.v1.CommunityService/ListPosts":      true,
			"/makejob.community.v1.CommunityService/GetPost":        true,
			"/makejob.community.v1.CommunityService/ListComments":   true,
		},
	}
	for _, opt := range opts {
		opt(i)
	}
	return i
}

// Option 拦截器配置选项
type Option func(*Interceptor)

// WithBlacklistChecker 注入 token 黑名单检查器
func WithBlacklistChecker(checker BlacklistChecker) Option {
	return func(i *Interceptor) {
		i.blacklist = checker
	}
}

// WithLogger 注入日志器
func WithLogger(logger log.Logger) Option {
	return func(i *Interceptor) {
		i.logger = log.NewHelper(logger)
	}
}

// UnaryServerInterceptor 返回 gRPC 一元服务器拦截器
func (i *Interceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// 公开方法跳过认证
		if i.publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		// 从 metadata 提取 token
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeaders := md.Get("authorization")
		if len(authHeaders) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		tokenString := strings.TrimPrefix(authHeaders[0], "Bearer ")
		if tokenString == authHeaders[0] {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
		}

		claims, err := ParseToken(tokenString, i.secret)
		if err != nil {
			// 不泄露内部错误详情给客户端
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		// FIX B1: 检查 token 是否在黑名单中（Logout 后立即失效）
		if i.blacklist != nil && claims.ID != "" {
			blocked, bErr := i.blacklist.IsBlacklisted(ctx, claims.ID)
			if bErr != nil {
				// Redis 不可用时降级放行，仅记录日志
				if i.logger != nil {
					i.logger.Warnw("msg", "黑名单检查失败，降级放行", "err", bErr)
				}
			} else if blocked {
				return nil, status.Error(codes.Unauthenticated, "token has been revoked")
			}
		}

		// 将用户信息注入 context
		ctx = context.WithValue(ctx, ContextKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, ContextKeyRole, claims.Role)
		ctx = context.WithValue(ctx, ContextKeyEmail, claims.Email)
		ctx = WithAccessToken(ctx, tokenString)

		return handler(ctx, req)
	}
}

// UnaryClientInterceptor 返回 gRPC 一元客户端拦截器（用于服务间调用）
func ServiceAuthInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
