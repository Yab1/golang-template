package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func SchemaCurrent(ctx context.Context, db *pgxpool.Pool, expectedVersion int) error {
	var version int
	var dirty bool
	if err := db.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &dirty); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if dirty {
		return fmt.Errorf("database schema version %d is dirty", version)
	}
	if version < expectedVersion {
		return fmt.Errorf("database schema version %d is behind required %d", version, expectedVersion)
	}
	return nil
}
