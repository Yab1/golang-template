package config

import "github.com/Yab1/golang-template/internal/platform/env"

type DB struct {
	Addr            string
	MaxConns        int
	MinConns        int
	MaxConnIdleTime string
	ExpectedVersion int
}

func loadDB() DB {
	return DB{
		Addr:            env.GetString("DB_ADDR", "postgres://admin:password@localhost:5432/golang_template?sslmode=disable"),
		MaxConns:        env.GetInt("DB_MAX_CONNS", 30),
		MinConns:        env.GetInt("DB_MIN_CONNS", 2),
		MaxConnIdleTime: env.GetString("DB_MAX_CONN_IDLE_TIME", "15m"),
		ExpectedVersion: env.GetInt("DB_EXPECTED_SCHEMA_VERSION", 6),
	}
}
