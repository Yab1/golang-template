package config

import "github.com/Yab1/golang-template/internal/platform/env"

type Seed struct {
	Enabled  bool
	Email    string
	Username string
	Password string
	Role     string
}

func loadSeed() Seed {
	return Seed{
		Enabled:  env.GetBool("SEED_ENABLED", false),
		Email:    env.GetString("SEED_ADMIN_EMAIL", "admin@localhost"),
		Username: env.GetString("SEED_ADMIN_USERNAME", "admin"),
		Password: env.GetString("SEED_ADMIN_PASSWORD", ""),
		Role:     env.GetString("SEED_ADMIN_ROLE", "admin"),
	}
}
