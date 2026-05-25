package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"makejob-backend/internal/common"
	"makejob-backend/pkg/logger"
)

// Recovery 自定义 panic 恢复中间件，替代 gin.Recovery()。
// panic 时通过 zap 输出结构化日志（含 request_id、stack trace），并返回标准 500 响应。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())

				fields := []zap.Field{
					zap.Any("error", r),
					zap.String("stack", stack),
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.String("ip", c.ClientIP()),
				}

				if rid := GetRequestID(c); rid != "" {
					fields = append(fields, zap.String("request_id", rid))
				}
				if uid, ok := GetUserID(c); ok {
					fields = append(fields, zap.Uint("user_id", uid))
				}

				logger.Error("HTTP panic recovered", fields...)

				common.InternalError(c, "服务器内部错误")
				c.Abort()
			}
		}()

		c.Next()
	}
}

// RecoveryWithWriter 支持自定义 writer 的 panic 恢复中间件。
// 当 logger 未初始化时可降级到 fmt.Fprintf。
func RecoveryWithWriter() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()

				// 尝试走 zap；如果 logger 未初始化则降级到 stderr
				if logger.Get() != nil {
					fields := []zap.Field{
						zap.Any("error", r),
						zap.String("stack", string(stack)),
						zap.String("method", c.Request.Method),
						zap.String("path", c.Request.URL.Path),
					}
					if rid := GetRequestID(c); rid != "" {
						fields = append(fields, zap.String("request_id", rid))
					}
					logger.Error("HTTP panic recovered", fields...)
				} else {
					fmt.Printf("[PANIC] %v\n%s\n", r, stack)
				}

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    common.CodeInternalError,
					"message": "服务器内部错误",
				})
			}
		}()

		c.Next()
	}
}
