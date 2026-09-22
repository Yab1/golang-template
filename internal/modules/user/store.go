package user

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yab1/golang-template/internal/platform/refid"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

const RefCode = "USR"

const userColumns = `
	u.id, u.reference_id, u.email, u.username, u.password, COALESCE(u.is_active, false),
	u.role_id, u.created_at, u.updated_at, u.created_by, u.updated_by,
	COALESCE(u.token_version, 1),
	r.id, r.name, r.level, COALESCE(r.description, '')
`

type Store struct {
	db   *pgxpool.Pool
	refs *refid.Generator
}

func NewStore(db *pgxpool.Pool, refs *refid.Generator) *Store {
	return &Store{db: db, refs: refs}
}

func (s *Store) Create(ctx context.Context, u *User) error {
	query := `
		INSERT INTO users (email, username, password, role_id, reference_id, is_active, created_by, updated_by)
		VALUES (
			$1,
			$2,
			$3,
			(SELECT id FROM roles WHERE name = $4),
			$5,
			$6,
			$7,
			$8
		)
		RETURNING id, role_id, created_at, updated_at, created_by, updated_by
	`

	roleName := u.Role.Name
	if roleName == "" {
		roleName = "user"
	}

	var lastErr error
	for i := 0; i < refid.MaxTries(); i++ {
		ref, err := s.refs.Next(RefCode)
		if err != nil {
			return err
		}
		u.ReferenceID = ref

		ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
		err = s.db.QueryRow(
			ctx,
			query,
			u.Email,
			u.Username,
			u.Password.hash,
			roleName,
			u.ReferenceID,
			u.IsActive,
			u.CreatedBy,
			u.UpdatedBy,
		).Scan(
			&u.ID,
			&u.RoleID,
			&u.CreatedAt,
			&u.UpdatedAt,
			&u.CreatedBy,
			&u.UpdatedBy,
		)
		cancel()
		if err == nil {
			if u.CreatedBy == nil || u.UpdatedBy == nil {
				if err := s.stampSelf(ctx, u.ID); err != nil {
					return err
				}
			}
			created, err := s.GetByID(ctx, u.ID)
			if err != nil {
				return err
			}
			*u = *created
			return nil
		}
		lastErr = err
		if refid.IsConflict(err) {
			continue
		}
		return storage.MapError(err)
	}

	if refid.IsConflict(lastErr) {
		return refid.ErrExhausted
	}
	return storage.MapError(lastErr)
}

// stampSelf sets created_by/updated_by to the user id after self-registration.
func (s *Store) stampSelf(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE users
		SET created_by = COALESCE(created_by, id),
		    updated_by = COALESCE(updated_by, id)
		WHERE id = $1
	`
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	_, err := s.db.Exec(ctx, query, id)
	return err
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.getUser(ctx, `
		SELECT `+userColumns+`
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.id = $1
	`, id)
}

func (s *Store) GetByEmail(ctx context.Context, email string) (*User, error) {
	return s.getUser(ctx, `
		SELECT `+userColumns+`
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.email = $1
	`, email)
}

func (s *Store) getUser(ctx context.Context, query string, arg any) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	var u User
	err := s.db.QueryRow(ctx, query, arg).Scan(
		&u.ID,
		&u.ReferenceID,
		&u.Email,
		&u.Username,
		&u.Password.hash,
		&u.IsActive,
		&u.RoleID,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.CreatedBy,
		&u.UpdatedBy,
		&u.TokenVersion,
		&u.Role.ID,
		&u.Role.Name,
		&u.Role.Level,
		&u.Role.Description,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}

	return &u, nil
}

func (s *Store) Activate(ctx context.Context, id uuid.UUID, updatedBy *uuid.UUID) error {
	query := `
		UPDATE users
		SET is_active = true, updated_at = NOW(), updated_by = COALESCE($2, updated_by)
		WHERE id = $1 AND is_active = false
	`
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	_, err := s.db.Exec(ctx, query, id, updatedBy)
	return err
}

func (s *Store) UpdatePassword(ctx context.Context, id uuid.UUID, hash []byte, updatedBy *uuid.UUID) error {
	query := `
		UPDATE users
		SET password = $2, updated_at = NOW(), updated_by = COALESCE($3, updated_by)
		WHERE id = $1
	`
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	tag, err := s.db.Exec(ctx, query, id, hash, updatedBy)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *Store) IncrementTokenVersion(ctx context.Context, id uuid.UUID) (int, error) {
	query := `
		UPDATE users
		SET token_version = token_version + 1, updated_at = NOW()
		WHERE id = $1
		RETURNING token_version
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	var version int
	err := s.db.QueryRow(ctx, query, id).Scan(&version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, storage.ErrNotFound
		}
		return 0, err
	}
	return version, nil
}
