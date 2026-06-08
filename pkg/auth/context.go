package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc/metadata"
)

// ContextKeyAccessToken 保存当前请求的 Bearer Token，供服务间透传使用。
const ContextKeyAccessToken ContextKey = "access_token"

// WithAccessToken 将当前请求的 Bearer Token 写入上下文。
func WithAccessToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ContextKeyAccessToken, token)
}

// GetAccessTokenFromContext 从上下文中读取 Bearer Token。
func GetAccessTokenFromContext(ctx context.Context) string {
	if v := ctx.Value(ContextKeyAccessToken); v != nil {
		if token, ok := v.(string); ok {
			return token
		}
	}
	return ""
}

// GetAccessTokenFromMetadata 从 gRPC 元数据中提取 Bearer Token 原文。
func GetAccessTokenFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(values[0], "Bearer "))
}

// WithOutgoingAccessToken 将 Bearer Token 追加到 gRPC 出站元数据，供服务间受保护 RPC 透传鉴权。
func WithOutgoingAccessToken(ctx context.Context, token string) context.Context {
	token = strings.TrimSpace(token)
	if token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}

// ForwardAccessToken 将当前上下文中的访问令牌补到 gRPC 出站元数据，优先读取拦截器写入的上下文值。
func ForwardAccessToken(ctx context.Context) context.Context {
	token := GetAccessTokenFromContext(ctx)
	if token == "" {
		token = GetAccessTokenFromMetadata(ctx)
	}
	return WithOutgoingAccessToken(ctx, token)
}
