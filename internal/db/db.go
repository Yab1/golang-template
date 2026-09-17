package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func New(addr string, maxConns, minConns int, maxConnIdleTime string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(addr)
	if err != nil {
		return nil, err
	}

	config.MaxConns = int32(maxConns)
	config.MinConns = int32(minConns)

	duration, err := time.ParseDuration(maxConnIdleTime)
	if err != nil {
		return nil, err
	}
	config.MaxConnIdleTime = duration

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
