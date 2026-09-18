package config

import (
	"time"

	"github.com/google/uuid"

	"github.com/Yab1/golang-template/internal/platform/env"
)

type Auth struct {
	Required            bool
	RBACEnabled         bool
	EmailVerifyRequired bool
	VerifyTokenExp      time.Duration
	ResetTokenExp       time.Duration
	DevToken            string
	DevUserID           uuid.UUID
	Token               Token
}

type Token struct {
	Secret     string
	Exp        time.Duration
	RefreshExp time.Duration
	Iss        string
	Aud        string
}

func loadAuth(envName string) Auth {
	devToken := env.GetString("AUTH_DEV_TOKEN", "")
	if envName != "development" {
		devToken = ""
	}

	return Auth{
		Required:            env.GetBool("AUTH_REQUIRED", true),
		RBACEnabled:         env.GetBool("RBAC_ENABLED", true),
		EmailVerifyRequired: env.GetBool("AUTH_EMAIL_VERIFY_REQUIRED", true),
		VerifyTokenExp:      env.GetDuration("AUTH_VERIFY_TOKEN_EXP", "24h"),
		ResetTokenExp:       env.GetDuration("AUTH_RESET_TOKEN_EXP", "1h"),
		DevToken:            devToken,
		DevUserID:           parseDevUserID(),
		Token: Token{
			Secret:     env.GetString("AUTH_TOKEN_SECRET", "dev-secret-change-me"),
			Exp:        env.GetDuration("AUTH_TOKEN_EXP", "15m"),
			RefreshExp: env.GetDuration("AUTH_REFRESH_EXP", "168h"),
			Iss:        env.GetString("AUTH_TOKEN_ISS", "golang-template"),
			Aud:        env.GetString("AUTH_TOKEN_AUD", "golang-template"),
		},
	}
}
