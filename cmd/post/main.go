package main

import (
	"context"
	"time"

	"github.com/Yab1/golang-template/internal/modules/user"
	"github.com/Yab1/golang-template/internal/platform/config"
	"github.com/Yab1/golang-template/internal/platform/db"
	"github.com/Yab1/golang-template/internal/platform/logger"
	"github.com/Yab1/golang-template/internal/platform/refid"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

func main() {
	cfg := config.Load()
	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()

	pool, err := db.New(cfg.DB.Addr, cfg.DB.MaxConns, cfg.DB.MinConns, cfg.DB.MaxConnIdleTime)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := storage.SchemaCurrent(ctx, pool, cfg.DB.ExpectedVersion); err != nil {
		log.Fatalw("database schema is not ready", "error", err)
	}
	if !cfg.Seed.Enabled {
		log.Info("POST complete; seeding disabled")
		return
	}
	u, created, err := user.SeedAdmin(
		ctx,
		user.NewStore(pool, refid.New(cfg.RefPrefix)),
		cfg.Seed.Email,
		cfg.Seed.Username,
		cfg.Seed.Password,
		cfg.Seed.Role,
	)
	if err != nil {
		log.Fatalw("POST seed failed", "error", err)
	}
	log.Infow(
		"POST complete",
		"seed_created", created,
		"user_id", u.ID,
		"reference_id", u.ReferenceID,
	)
}
