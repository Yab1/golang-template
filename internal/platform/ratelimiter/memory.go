package ratelimiter

import (
	"context"
	"sync"
	"time"
)

type MemoryLimiter struct {
	mu      sync.Mutex
	clients map[string]*memoryBucket
}

type memoryBucket struct {
	count     int
	expiresAt time.Time
}

func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{
		clients: make(map[string]*memoryBucket),
	}
}

func (l *MemoryLimiter) Allow(_ context.Context, key string, rule Rule) (Result, error) {
	now := time.Now()
	window := rule.Window
	if window <= 0 {
		window = time.Minute
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, ok := l.clients[key]
	if !ok || now.After(bucket.expiresAt) {
		bucket = &memoryBucket{
			count:     0,
			expiresAt: now.Add(window),
		}
		l.clients[key] = bucket
	}

	retryAfter := bucket.expiresAt.Sub(now)
	if retryAfter < 0 {
		retryAfter = 0
	}

	if bucket.count >= rule.Requests {
		return Result{
			Allowed:    false,
			Limit:      rule.Requests,
			Remaining:  0,
			RetryAfter: retryAfter,
		}, nil
	}

	bucket.count++
	remaining := rule.Requests - bucket.count
	if remaining < 0 {
		remaining = 0
	}

	return Result{
		Allowed:    true,
		Limit:      rule.Requests,
		Remaining:  remaining,
		RetryAfter: retryAfter,
	}, nil
}
