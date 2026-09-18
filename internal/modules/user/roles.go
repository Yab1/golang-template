package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yab1/golang-template/internal/platform/storage"
)

type RoleStore struct {
	db *pgxpool.Pool
}

func NewRoleStore(db *pgxpool.Pool) *RoleStore {
	return &RoleStore{db: db}
}

func (s *RoleStore) GetByName(ctx context.Context, name string) (*Role, error) {
	query := `
		SELECT id, name, level, description
		FROM roles
		WHERE name = $1
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	var role Role
	err := s.db.QueryRow(ctx, query, name).Scan(
		&role.ID,
		&role.Name,
		&role.Level,
		&role.Description,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}

	return &role, nil
}
