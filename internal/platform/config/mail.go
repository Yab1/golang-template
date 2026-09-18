package config

import (
	"strings"

	"github.com/Yab1/golang-template/internal/platform/env"
)

type Mail struct {
	Enabled  bool
	Driver   string
	From     string
	FromName string
	SMTP     SMTP
	SES      SES
}

type SMTP struct {
	Host     string
	Port     int
	User     string
	Pass     string
	TLS      bool
	StartTLS bool
}

type SES struct {
	Region    string
	AccessKey string
	SecretKey string
}

func loadMail(appName string) Mail {
	return Mail{
		Enabled:  env.GetBool("MAIL_ENABLED", true),
		Driver:   strings.ToLower(env.GetString("MAIL_DRIVER", "log")),
		From:     env.GetString("MAIL_FROM", "noreply@localhost"),
		FromName: env.GetString("MAIL_FROM_NAME", appName),
		SMTP: SMTP{
			Host:     env.GetString("SMTP_HOST", "localhost"),
			Port:     env.GetInt("SMTP_PORT", 1025),
			User:     env.GetString("SMTP_USER", ""),
			Pass:     env.GetString("SMTP_PASS", ""),
			TLS:      env.GetBool("SMTP_TLS", false),
			StartTLS: env.GetBool("SMTP_STARTTLS", false),
		},
		SES: SES{
			Region:    env.GetString("SES_REGION", "us-east-1"),
			AccessKey: env.GetString("SES_ACCESS_KEY", ""),
			SecretKey: env.GetString("SES_SECRET_KEY", ""),
		},
	}
}
