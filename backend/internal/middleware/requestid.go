package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type requestIDContextKey string

const (
	// RequestIDContextKey 是 std context 中 request_id 的键。
	RequestIDContextKey requestIDContextKey = "request_id"

	// GinKeyRequestID 是 Gin context 中 request_id 的键。
	GinKeyRequestID = "request_id"

	// HeaderRequestID 是 HTTP 请求/响应头中的 request_id 键。
	HeaderRequestID = "X-Request-ID"
)

// RequestID 生成或透传请求唯一标识，注入 Gin context 和 std context。
// 放在中间件链最前面，确保后续所有层都能读到。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(HeaderRequestID)
		if rid == "" {
			rid = uuid.NewString()
		}

		c.Set(GinKeyRequestID, rid)
		c.Header(HeaderRequestID, rid)

		ctx := context.WithValue(c.Request.Context(), RequestIDContextKey, rid)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// GetRequestID 从 Gin context 中读取 request_id。
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(GinKeyRequestID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetRequestIDFromContext 从 std context 中读取 request_id。
func GetRequestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(RequestIDContextKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
