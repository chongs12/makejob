// Package ratelimit 提供基于 Redis 的分布式限流能力。
package ratelimit

// SlideWindowScript 滑动窗口限流 Lua 脚本。
// 使用 Redis ZSET 实现滑动窗口，保证原子性操作。
//
// KEYS[1] = 限流 key
// ARGV[1] = 窗口大小（秒）
// ARGV[2] = 最大请求数
// ARGV[3] = 当前时间戳（微秒）
//
// 返回: {allowed(0/1), remaining, retry_after_ms}
const SlideWindowScript = `
local key = KEYS[1]
local window = tonumber(ARGV[1])
local max_requests = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local window_start = now - window * 1000000

-- 移除窗口外的请求记录
redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

-- 获取当前窗口内的请求数
local current = redis.call('ZCARD', key)

if current < max_requests then
    -- 允许请求，记录当前请求
    local member = now .. '-' .. math.random(1000000)
    redis.call('ZADD', key, now, member)
    redis.call('EXPIRE', key, window)
    return {1, max_requests - current - 1, 0}
else
    -- 拒绝请求，计算重试等待时间
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local retry_after = 0
    if #oldest >= 2 then
        retry_after = math.ceil((tonumber(oldest[2]) + window * 1000000 - now) / 1000)
    end
    return {0, 0, retry_after}
end
`
