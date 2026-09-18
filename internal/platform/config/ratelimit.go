package config

import (
	"time"

	"github.com/Yab1/golang-template/internal/platform/env"
)

type RateLimit struct {
	Enabled bool
	Auth    Rule
	Write   Rule
	Read    Rule
}

type Rule struct {
	Requests int
	Window   time.Duration
}

func loadRateLimit() RateLimit {
	return RateLimit{
		Enabled: env.GetBool("RATE_LIMIT_ENABLED", true),
		Auth:    minuteRule(env.GetInt("RATE_LIMIT_AUTH", 10)),
		Write:   minuteRule(env.GetInt("RATE_LIMIT_WRITE", 120)),
		Read:    minuteRule(env.GetInt("RATE_LIMIT_READ", 600)),
	}
}
