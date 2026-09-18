package config

import "github.com/Yab1/golang-template/internal/platform/env"

type Audit struct {
	Enabled bool
}

func loadAudit() Audit {
	return Audit{
		Enabled: env.GetBool("AUDIT_ENABLED", true),
	}
}
