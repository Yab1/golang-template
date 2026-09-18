package config

import "github.com/Yab1/golang-template/internal/platform/env"

type Metrics struct {
	Enabled bool
	Token   string
}

func loadMetrics() Metrics {
	return Metrics{
		Enabled: env.GetBool("METRICS_ENABLED", false),
		Token:   env.GetString("METRICS_TOKEN", ""),
	}
}
