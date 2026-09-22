package file

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yab1/golang-template/internal/platform/storage"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) CreateTx(ctx context.Context, tx pgx.Tx, object *Object) error {
	query := `
		INSERT INTO file_objects (
			key, owner_id, content_type, size_bytes, driver, is_visible,
			metadata, created_by, updated_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING version, created_at, updated_at
	`
	return tx.QueryRow(
		ctx,
		query,
		object.Key,
		object.OwnerID,
		object.ContentType,
		object.Size,
		object.Driver,
		object.IsVisible,
		object.Metadata,
		object.CreatedBy,
		object.UpdatedBy,
	).Scan(&object.Version, &object.CreatedAt, &object.UpdatedAt)
}

func (s *Store) Get(ctx context.Context, key string) (*Object, error) {
	query := `
		SELECT
			key, owner_id, content_type, size_bytes, driver, version, is_visible, metadata,
			created_by, updated_by, deleted_by, deleted_at, created_at, updated_at
		FROM file_objects
		WHERE key = $1 AND deleted_at IS NULL AND is_visible
	`
	var object Object
	err := s.db.QueryRow(ctx, query, key).Scan(
		&object.Key,
		&object.OwnerID,
		&object.ContentType,
		&object.Size,
		&object.Driver,
		&object.Version,
		&object.IsVisible,
		&object.Metadata,
		&object.CreatedBy,
		&object.UpdatedBy,
		&object.DeletedBy,
		&object.DeletedAt,
		&object.CreatedAt,
		&object.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	return &object, nil
}

func (s *Store) SoftDeleteTx(ctx context.Context, tx pgx.Tx, object *Object, actor *uuid.UUID) error {
	err := tx.QueryRow(ctx, `
		UPDATE file_objects
		SET deleted_at = NOW(),
		    deleted_by = $2,
		    updated_by = $2,
		    updated_at = NOW(),
		    version = version + 1
		WHERE key = $1 AND deleted_at IS NULL
		RETURNING version
	`, object.Key, actor).Scan(&object.Version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return storage.ErrNotFound
		}
		return err
	}
	return nil
}
