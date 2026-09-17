package main

import (
	"github.com/Yab1/golang-template/internal/db"
	"github.com/Yab1/golang-template/internal/env"
	"github.com/Yab1/golang-template/internal/store"
	"go.uber.org/zap"
)

const version = "0.0.1"

func main() {
	cfg := config{
		addr: env.GetString("ADDR", ":8080"),
		env:  env.GetString("ENV", "development"),
		db: dbConfig{
			addr:            env.GetString("DB_ADDR", "postgres://admin:password@localhost:5432/golang_template?sslmode=disable"),
			maxConns:        env.GetInt("DB_MAX_CONNS", 30),
			minConns:        env.GetInt("DB_MIN_CONNS", 2),
			maxConnIdleTime: env.GetString("DB_MAX_CONN_IDLE_TIME", "15m"),
		},
	}

	logger := zap.Must(zap.NewProduction()).Sugar()
	defer logger.Sync()

	db, err := db.New(
		cfg.db.addr,
		cfg.db.maxConns,
		cfg.db.minConns,
		cfg.db.maxConnIdleTime,
	)
	if err != nil {
		logger.Fatal(err)
	}
	defer db.Close()
	logger.Info("database connection pool established")

	store := store.NewStorage(db)

	app := &application{
		config: cfg,
		store:  store,
		logger: logger,
	}

	mux := app.mount()

	logger.Fatal(app.run(mux))
}
