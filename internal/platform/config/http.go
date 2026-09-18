package config

import (
	"time"

	"github.com/Yab1/golang-template/internal/platform/env"
)

type HTTP struct {
	CORSEnabled     bool
	CORSOrigins     []string
	ShutdownTimeout time.Duration
	TrustedProxies  []string
	PublicBaseURL   string
}

func loadHTTP() HTTP {
	return HTTP{
		CORSEnabled:     env.GetBool("CORS_ENABLED", true),
		CORSOrigins:     env.GetCSV("CORS_ALLOWED_ORIGINS", []string{"*"}),
		ShutdownTimeout: env.GetDuration("SHUTDOWN_TIMEOUT", "10s"),
		TrustedProxies:  env.GetCSV("TRUSTED_PROXIES", nil),
		PublicBaseURL:   env.GetString("PUBLIC_BASE_URL", "http://localhost:8080"),
	}
}
