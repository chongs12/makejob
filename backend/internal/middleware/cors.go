// Package middleware 提供Gin中间件功能
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS 跨域资源共享中间件
// 开发阶段允许所有来源，生产环境应配置具体域名
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 允许所有来源
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		// 允许的HTTP方法
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		// 允许的请求头
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Accept, Cache-Control, X-Requested-With")
		// 允许携带凭证（如Cookies）
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		// 预检请求缓存时间（秒）
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
