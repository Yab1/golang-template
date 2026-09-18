package config

import "github.com/Yab1/golang-template/internal/platform/env"

type Log struct {
	Level  string
	Format string
}

func loadLog(envName string) Log {
	format := "json"
	level := "info"
	if envName == "development" {
		format = "console"
		level = "debug"
	}
	return Log{
		Level:  env.GetString("LOG_LEVEL", level),
		Format: env.GetString("LOG_FORMAT", format),
	}
}
