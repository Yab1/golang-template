package config

import "github.com/Yab1/golang-template/internal/platform/env"

type Redis struct {
	Addr     string
	Password string
	DB       int
	Enabled  bool
}

func loadRedis() Redis {
	return Redis{
		Addr:     env.GetString("REDIS_ADDR", "localhost:6379"),
		Password: env.GetString("REDIS_PW", ""),
		DB:       env.GetInt("REDIS_DB", 0),
		Enabled:  env.GetBool("REDIS_ENABLED", true),
	}
}
