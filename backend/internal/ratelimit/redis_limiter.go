package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"makejob-backend/internal/config"
)

// RedisLimiter 基于 Redis 滑动窗口的分布式限流器。
type RedisLimiter struct {
	rdb    *redis.Client
	script *redis.Script
	rules  map[string]config.RateLimitRuleConfig
	mu     sync.RWMutex
}

// AllowResult 限流检查结果。
type AllowResult struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

// NewRedisLimiter 创建分布式限流器。
func NewRedisLimiter(rdb *redis.Client, rules []config.RateLimitRuleConfig) *RedisLimiter {
	ruleMap := make(map[string]config.RateLimitRuleConfig, len(rules))
	for _, rule := range rules {
		ruleMap[rule.Name] = rule
	}

	return &RedisLimiter{
		rdb:    rdb,
		script: redis.NewScript(SlideWindowScript),
		rules:  ruleMap,
	}
}

// GetRule 获取指定名称的限流规则。
func (l *RedisLimiter) GetRule(name string) (config.RateLimitRuleConfig, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	rule, ok := l.rules[name]
	return rule, ok
}

// Allow 检查指定 key 是否允许通过。
func (l *RedisLimiter) Allow(ctx context.Context, ruleName, identifier string) AllowResult {
	rule, ok := l.GetRule(ruleName)
	if !ok {
		return AllowResult{Allowed: true}
	}

	key := l.buildKey(ruleName, identifier)

	// 计算窗口大小（秒），保护除零
	var window int64
	if rule.Rate > 0 {
		window = int64(float64(rule.Capacity) / rule.Rate)
	}
	if window < 1 {
		window = 1
	}

	now := time.Now().UnixMicro()

	result, err := l.script.Run(ctx, l.rdb, []string{key},
		window,
		rule.Capacity,
		now,
	).Int64Slice()

	if err != nil {
		// Redis 调用失败时默认放行
		return AllowResult{Allowed: true}
	}

	if len(result) < 3 {
		return AllowResult{Allowed: true}
	}

	return AllowResult{
		Allowed:    result[0] == 1,
		Remaining:  int(result[1]),
		RetryAfter: time.Duration(result[2]) * time.Millisecond,
	}
}

// AllowByIP 按 IP 限流的便捷方法。
func (l *RedisLimiter) AllowByIP(ctx context.Context, ruleName, ip string) AllowResult {
	return l.Allow(ctx, ruleName, fmt.Sprintf("ip:%s", ip))
}

// AllowByUserID 按用户 ID 限流的便捷方法。
func (l *RedisLimiter) AllowByUserID(ctx context.Context, ruleName string, userID uint) AllowResult {
	return l.Allow(ctx, ruleName, fmt.Sprintf("uid:%d", userID))
}

// buildKey 构建 Redis key。
// 格式: ratelimit:{rule_name}:{identifier}
func (l *RedisLimiter) buildKey(ruleName, identifier string) string {
	return fmt.Sprintf("ratelimit:%s:%s", ruleName, identifier)
}
