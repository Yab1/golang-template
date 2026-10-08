package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yab1/golang-template/internal/platform/storage"
)

// Role name slugs — must match roles.name in 000001_create_users.
const (
	RoleUser      = "user"
	RoleModerator = "moderator"
	RoleAdmin     = "admin"

	// DefaultRole is assigned on public register when no role is set.
	DefaultRole = RoleUser
)

// Role levels — must match roles.level in 000001_create_users.
const (
	LevelUser      = 1
	LevelModerator = 2
	LevelAdmin     = 3
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

func (s *RoleStore) List(ctx context.Context) ([]Role, error) {
	query := `
		SELECT id, name, level, description
		FROM roles
		ORDER BY level DESC, name ASC
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Level, &role.Description); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if roles == nil {
		roles = []Role{}
	}
	return roles, nil
}

func (s *RoleStore) GetByID(ctx context.Context, id int64) (*Role, error) {
	query := `
		SELECT id, name, level, description
		FROM roles
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	var role Role
	err := s.db.QueryRow(ctx, query, id).Scan(
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

func (s *RoleStore) Create(ctx context.Context, role *Role) error {
	query := `
		INSERT INTO roles (name, level, description)
		VALUES ($1, $2, $3)
		RETURNING id
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	err := s.db.QueryRow(ctx, query, role.Name, role.Level, role.Description).Scan(&role.ID)
	if err != nil {
		return storage.MapError(err)
	}
	return nil
}

func (s *RoleStore) Update(ctx context.Context, role *Role) error {
	query := `
		UPDATE roles
		SET name = $2,
		    level = $3,
		    description = $4
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	tag, err := s.db.Exec(ctx, query, role.ID, role.Name, role.Level, role.Description)
	if err != nil {
		return storage.MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *RoleStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM roles WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	tag, err := s.db.Exec(ctx, query, id)
	if err != nil {
		return storage.MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *RoleStore) CountUsers(ctx context.Context, roleID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM users WHERE role_id = $1`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	var n int64
	if err := s.db.QueryRow(ctx, query, roleID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
