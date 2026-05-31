package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"makejob-backend/internal/common"
	"makejob-backend/internal/config"
	"makejob-backend/internal/ratelimit"
	"makejob-backend/pkg/logger"
)

// DistributedRateLimit 创建分布式限流中间件（使用默认规则）。
// 当 rdb 为 nil 或 cfg.Enabled 为 false 时，自动降级到本地令牌桶限流。
func DistributedRateLimit(rdb *redis.Client, cfg config.DistributedRateLimitConfig) gin.HandlerFunc {
	return DistributedRateLimitByRule(rdb, cfg, "default")
}

// DistributedRateLimitByRule 按指定规则名创建分布式限流中间件。
func DistributedRateLimitByRule(rdb *redis.Client, cfg config.DistributedRateLimitConfig, ruleName string) gin.HandlerFunc {
	if rdb == nil || !cfg.Enabled {
		return fallbackLocalRateLimit(cfg, ruleName)
	}

	limiter := ratelimit.NewRedisLimiter(rdb, cfg.Rules)
	rule, ok := limiter.GetRule(ruleName)
	if !ok {
		logger.Warn("rate limit rule not found, allowing all requests", zap.String("rule", ruleName))
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		// 根据规则配置选择限流维度
		var identifier string
		if rule.ByKey == "user_id" {
			if uid, exists := GetUserID(c); exists {
				identifier = fmt.Sprintf("uid:%d", uid)
			}
		}
		// 默认按 IP 限流（identifier 为空时 Allow 方法会使用纯 IP）
		if identifier == "" {
			identifier = c.ClientIP()
		}

		result := limiter.Allow(c.Request.Context(), ruleName, identifier)

		// 设置标准限流响应头
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rule.Capacity))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))

		if !result.Allowed {
			if result.RetryAfter > 0 {
				c.Header("Retry-After", fmt.Sprintf("%d", int(result.RetryAfter.Seconds())))
			}
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

// StrictDistributedRateLimit 严格分布式限流（登录/注册等敏感接口）。
func StrictDistributedRateLimit(rdb *redis.Client, cfg config.DistributedRateLimitConfig) gin.HandlerFunc {
	return DistributedRateLimitByRule(rdb, cfg, "strict")
}

// PublicDistributedRateLimit 公开接口分布式限流。
func PublicDistributedRateLimit(rdb *redis.Client, cfg config.DistributedRateLimitConfig) gin.HandlerFunc {
	return DistributedRateLimitByRule(rdb, cfg, "public")
}

// fallbackLocalRateLimit 降级到本地限流。
func fallbackLocalRateLimit(cfg config.DistributedRateLimitConfig, ruleName string) gin.HandlerFunc {
	for _, rule := range cfg.Rules {
		if rule.Name == ruleName {
			logger.Warn("falling back to local rate limiter",
				zap.String("rule", ruleName),
				zap.Float64("rate", rule.Rate),
				zap.Int("capacity", rule.Capacity),
			)
			return RateLimitWithParams(rule.Rate, rule.Capacity)
		}
	}
	return RateLimit()
}
