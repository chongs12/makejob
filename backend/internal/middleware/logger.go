// Package middleware 提供Gin中间件功能
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"makejob-backend/pkg/logger"
)

// Logger 请求日志中间件
// 记录每个HTTP请求的方法、路径、状态码、耗时、request_id、user_id 等信息。
// 需放在 RequestID 中间件之后。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		method := c.Request.Method
		clientIP := c.ClientIP()

		// 处理请求
		c.Next()

		// 计算耗时
		duration := time.Since(start)
		statusCode := c.Writer.Status()

		// 构建日志字段
		fields := []zap.Field{
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Duration("duration", duration),
			zap.String("ip", clientIP),
		}

		// 从 context 注入的 request_id
		if rid := GetRequestID(c); rid != "" {
			fields = append(fields, zap.String("request_id", rid))
		}

		// Auth 中间件在 Logger 之前执行（per-group），此时 user_id 已可用
		if uid, ok := GetUserID(c); ok {
			fields = append(fields, zap.Uint("user_id", uid))
		}

		if raw != "" {
			fields = append(fields, zap.String("query", raw))
		}

		// 记录错误信息（如果有）
		if len(c.Errors) > 0 {
			fields = append(fields, zap.Strings("errors", c.Errors.Errors()))
		}

		// 根据状态码选择日志级别
		switch {
		case statusCode >= 500:
			logger.Error("HTTP请求错误", fields...)
		case statusCode >= 400:
			logger.Warn("HTTP请求警告", fields...)
		default:
			logger.Info("HTTP请求", fields...)
		}
	}
}

// LoggerWithSkipPaths 带跳过路径的日志中间件
// skipPaths 中的路径不会被记录
func LoggerWithSkipPaths(skipPaths []string) gin.HandlerFunc {
	skipMap := make(map[string]bool)
	for _, path := range skipPaths {
		skipMap[path] = true
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 检查是否需要跳过
		if skipMap[path] {
			c.Next()
			return
		}

		start := time.Now()
		raw := c.Request.URL.RawQuery
		method := c.Request.Method
		clientIP := c.ClientIP()

		c.Next()

		duration := time.Since(start)
		statusCode := c.Writer.Status()

		fields := []zap.Field{
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Duration("duration", duration),
			zap.String("ip", clientIP),
		}

		if rid := GetRequestID(c); rid != "" {
			fields = append(fields, zap.String("request_id", rid))
		}

		if uid, ok := GetUserID(c); ok {
			fields = append(fields, zap.Uint("user_id", uid))
		}

		if raw != "" {
			fields = append(fields, zap.String("query", raw))
		}

		if len(c.Errors) > 0 {
			fields = append(fields, zap.Strings("errors", c.Errors.Errors()))
		}

		switch {
		case statusCode >= 500:
			logger.Error("HTTP请求错误", fields...)
		case statusCode >= 400:
			logger.Warn("HTTP请求警告", fields...)
		default:
			logger.Info("HTTP请求", fields...)
		}
	}
}
