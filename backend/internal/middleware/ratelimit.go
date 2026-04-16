// Package middleware 提供Gin中间件功能
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"makejob-backend/internal/common"
	"makejob-backend/pkg/logger"
)

// RateLimiter 基于令牌桶算法的限流器
type RateLimiter struct {
	rate       float64   // 每秒产生令牌数
	capacity   int       // 桶容量
	tokens     float64   // 当前令牌数
	lastUpdate time.Time // 上次更新时间
	mu         sync.Mutex
}

// NewRateLimiter 创建新的限流器
// rate: 每秒产生令牌数
// capacity: 桶容量（最大突发流量）
func NewRateLimiter(rate float64, capacity int) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		capacity:   capacity,
		tokens:     float64(capacity),
		lastUpdate: time.Now(),
	}
}

// Allow 检查是否允许请求通过
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastUpdate).Seconds()
	rl.lastUpdate = now

	// 添加新产生的令牌
	rl.tokens += elapsed * rl.rate
	if rl.tokens > float64(rl.capacity) {
		rl.tokens = float64(rl.capacity)
	}

	// 检查是否有可用令牌
	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}

	return false
}

// IPRateLimiter 基于IP的限流器管理器
type IPRateLimiter struct {
	limiters map[string]*RateLimiter
	rate     float64
	capacity int
	mu       sync.RWMutex
}

// NewIPRateLimiter 创建基于IP的限流器
func NewIPRateLimiter(rate float64, capacity int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*RateLimiter),
		rate:     rate,
		capacity: capacity,
	}
}

// GetLimiter 获取或创建指定IP的限流器
func (rl *IPRateLimiter) GetLimiter(ip string) *RateLimiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[ip]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	// 双重检查
	if limiter, exists = rl.limiters[ip]; exists {
		rl.mu.Unlock()
		return limiter
	}

	limiter = NewRateLimiter(rl.rate, rl.capacity)
	rl.limiters[ip] = limiter
	rl.mu.Unlock()

	return limiter
}

// Cleanup 清理过期的限流器（可选，用于长时间运行的服务）
func (rl *IPRateLimiter) Cleanup(maxIdle time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, limiter := range rl.limiters {
		limiter.mu.Lock()
		idle := now.Sub(limiter.lastUpdate)
		limiter.mu.Unlock()

		if idle > maxIdle {
			delete(rl.limiters, ip)
		}
	}
}

var (
	// 默认全局IP限流器实例
	defaultIPLimiter = NewIPRateLimiter(100, 200) // 每秒100个请求，突发200个
)

// RateLimit 简单限流中间件
// 基于IP地址进行限流，使用令牌桶算法
func RateLimit() gin.HandlerFunc {
	return RateLimitWithConfig(defaultIPLimiter)
}

// RateLimitWithConfig 带配置的限流中间件
func RateLimitWithConfig(limiter *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		l := limiter.GetLimiter(ip)

		if !l.Allow() {
			logger.Warn("请求过于频繁",
				zap.String("ip", ip))
			c.JSON(http.StatusTooManyRequests, common.Response{
				Code:    429,
				Message: "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitWithParams 自定义参数的限流中间件
// rate: 每秒允许的请求数
// capacity: 桶容量（突发流量）
func RateLimitWithParams(rate float64, capacity int) gin.HandlerFunc {
	limiter := NewIPRateLimiter(rate, capacity)
	return RateLimitWithConfig(limiter)
}

// StrictRateLimit 严格限流中间件
// 适用于敏感接口，如登录、注册等
func StrictRateLimit() gin.HandlerFunc {
	// 每秒1个请求，突发3个
	limiter := NewIPRateLimiter(1, 3)
	return RateLimitWithConfig(limiter)
}

// PublicRateLimit 公开接口限流中间件
// 适用于普通公开接口，限制较宽松
func PublicRateLimit() gin.HandlerFunc {
	// 每秒20个请求，突发50个
	limiter := NewIPRateLimiter(20, 50)
	return RateLimitWithConfig(limiter)
}
