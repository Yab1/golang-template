package ratelimiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Sliding-window counter: current bucket + weighted previous bucket.
const slidingWindowLua = `
local window = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local current_bucket = math.floor(now / window)
local previous_bucket = current_bucket - 1
local current_key = KEYS[1] .. ":" .. current_bucket
local previous_key = KEYS[1] .. ":" .. previous_bucket

local current = tonumber(redis.call("GET", current_key) or "0")
local previous = tonumber(redis.call("GET", previous_key) or "0")

local elapsed = now % window
local weight = (window - elapsed) / window
local count = previous * weight + current
local retry = window - elapsed

if count >= limit then
  return {0, limit, 0, retry}
end

current = redis.call("INCR", current_key)
redis.call("PEXPIRE", current_key, window * 2)
count = previous * weight + current
local remaining = math.floor(limit - count)
if remaining < 0 then
  remaining = 0
end

return {1, limit, remaining, retry}
`

type RedisLimiter struct {
	client *redis.Client
	script *redis.Script
}

func NewRedisLimiter(client *redis.Client) *RedisLimiter {
	return &RedisLimiter{
		client: client,
		script: redis.NewScript(slidingWindowLua),
	}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string, rule Rule) (Result, error) {
	windowMs := rule.Window.Milliseconds()
	if windowMs <= 0 {
		windowMs = 1000
	}

	nowMs := time.Now().UnixMilli()
	vals, err := l.script.Run(ctx, l.client, []string{key}, windowMs, rule.Requests, nowMs).Int64Slice()
	if err != nil {
		return Result{}, err
	}

	allowed := vals[0] == 1
	limit := int(vals[1])
	remaining := int(vals[2])
	retryAfter := time.Duration(vals[3]) * time.Millisecond

	return Result{
		Allowed:    allowed,
		Limit:      limit,
		Remaining:  remaining,
		RetryAfter: retryAfter,
	}, nil
}
