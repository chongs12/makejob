// Package middleware 提供 Gin 中间件能力。
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// applyCORSHeaders 根据请求来源回写跨域响应头，兼容本地开发代理和跨域直连。
func applyCORSHeaders(c *gin.Context) {
	origin := strings.TrimSpace(c.Request.Header.Get("Origin"))
	if origin == "" {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		return
	}

	c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
	c.Writer.Header().Set("Vary", "Origin")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
}

// CORS 跨域资源共享中间件。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		applyCORSHeaders(c)
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Accept, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
