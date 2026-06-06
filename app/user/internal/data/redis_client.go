package data

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"makejob/app/user/internal/biz"
	"makejob/app/user/internal/conf"
)

// NewRedisClient 创建 Redis 客户端
func NewRedisClient(cfg *conf.Data_Redis) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}

// tokenBlacklist 基于 Redis 的 token 黑名单实现
type tokenBlacklist struct {
	rdb *redis.Client
}

// NewTokenBlacklist 创建 token 黑名单实例
func NewTokenBlacklist(rdb *redis.Client) biz.TokenBlacklist {
	return &tokenBlacklist{rdb: rdb}
}

// Add 将 token JTI 加入黑名单，设置自动过期
func (t *tokenBlacklist) Add(ctx context.Context, tokenJTI string, ttl time.Duration) error {
	key := fmt.Sprintf("token_blacklist:%s", tokenJTI)
	return t.rdb.Set(ctx, key, "1", ttl).Err()
}

// IsBlacklisted 检查 token JTI 是否在黑名单中
func (t *tokenBlacklist) IsBlacklisted(ctx context.Context, tokenJTI string) (bool, error) {
	key := fmt.Sprintf("token_blacklist:%s", tokenJTI)
	n, err := t.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
