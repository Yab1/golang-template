package ratelimiter

import (
	"context"
	"time"
)

type Rule struct {
	Requests int
	Window   time.Duration
}

type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration
}

type Limiter interface {
	Allow(ctx context.Context, key string, rule Rule) (Result, error)
}
