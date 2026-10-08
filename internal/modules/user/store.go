package user

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yab1/golang-template/internal/platform/query"
	"github.com/Yab1/golang-template/internal/platform/refid"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

const RefCode = "USR"

var emptyMeta = json.RawMessage(`{}`)

const userColumns = `
	u.id, u.reference_id, u.email, u.username, u.password, COALESCE(u.is_active, false),
	u.is_visible, u.metadata, u.role_id, u.version, u.created_at, u.updated_at, u.created_by, u.updated_by,
	COALESCE(u.token_version, 1),
	r.id, r.name, r.level, COALESCE(r.description, '')
`

type Store struct {
	db   *pgxpool.Pool
	refs *refid.Generator
}

type userDBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func NewStore(db *pgxpool.Pool, refs *refid.Generator) *Store {
	return &Store{db: db, refs: refs}
}

func metaOrEmpty(m json.RawMessage) json.RawMessage {
	if len(m) == 0 || string(m) == "null" {
		return emptyMeta
	}
	return m
}

func (s *Store) Create(ctx context.Context, u *User) error {
	return s.create(ctx, s.db, u)
}

func (s *Store) CreateTx(ctx context.Context, tx pgx.Tx, u *User) error {
	return s.create(ctx, tx, u)
}

func (s *Store) create(ctx context.Context, db userDBTX, u *User) error {
	query := `
		INSERT INTO users (email, username, password, role_id, reference_id, is_active, is_visible, metadata, created_by, updated_by)
		VALUES (
			$1,
			$2,
			$3,
			(SELECT id FROM roles WHERE name = $4),
			$5,
			$6,
			$7,
			$8,
			$9,
			$10
		)
		RETURNING id, role_id, version, created_at, updated_at, created_by, updated_by, is_visible, metadata
	`

	roleName := u.Role.Name
	if roleName == "" {
		roleName = "user"
	}
	u.Metadata = metaOrEmpty(u.Metadata)

	var lastErr error
	for i := 0; i < refid.MaxTries(); i++ {
		ref, err := s.refs.Next(RefCode)
		if err != nil {
			return err
		}
		u.ReferenceID = ref

		queryCtx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
		err = db.QueryRow(
			queryCtx,
			query,
			u.Email,
			u.Username,
			u.Password.hash,
			roleName,
			u.ReferenceID,
			u.IsActive,
			u.IsVisible,
			u.Metadata,
			u.CreatedBy,
			u.UpdatedBy,
		).Scan(
			&u.ID,
			&u.RoleID,
			&u.Version,
			&u.CreatedAt,
			&u.UpdatedAt,
			&u.CreatedBy,
			&u.UpdatedBy,
			&u.IsVisible,
			&u.Metadata,
		)
		cancel()
		if err == nil {
			u.Metadata = metaOrEmpty(u.Metadata)
			if u.CreatedBy == nil || u.UpdatedBy == nil {
				if err := s.stampSelf(ctx, db, u.ID); err != nil {
					return err
				}
			}
			created, err := s.getUserWith(ctx, db, `
				SELECT `+userColumns+`
				FROM users u
				JOIN roles r ON r.id = u.role_id
				WHERE u.id = $1
			`, u.ID)
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

func (s *Store) stampSelf(ctx context.Context, db userDBTX, id uuid.UUID) error {
	query := `
		UPDATE users
		SET created_by = COALESCE(created_by, id),
		    updated_by = COALESCE(updated_by, id)
		WHERE id = $1
	`
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	_, err := db.Exec(ctx, query, id)
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
	return s.getUserWith(ctx, s.db, query, arg)
}

func (s *Store) getUserWith(ctx context.Context, db userDBTX, query string, arg any) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	var u User
	err := db.QueryRow(ctx, query, arg).Scan(
		&u.ID,
		&u.ReferenceID,
		&u.Email,
		&u.Username,
		&u.Password.hash,
		&u.IsActive,
		&u.IsVisible,
		&u.Metadata,
		&u.RoleID,
		&u.Version,
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
	u.Metadata = metaOrEmpty(u.Metadata)

	return &u, nil
}

func (s *Store) Activate(ctx context.Context, id uuid.UUID, updatedBy *uuid.UUID) (bool, error) {
	return s.activate(ctx, s.db, id, updatedBy)
}

func (s *Store) ActivateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, updatedBy *uuid.UUID) (bool, error) {
	return s.activate(ctx, tx, id, updatedBy)
}

func (s *Store) activate(ctx context.Context, db userDBTX, id uuid.UUID, updatedBy *uuid.UUID) (bool, error) {
	query := `
		UPDATE users
		SET is_active = true,
		    version = version + 1,
		    updated_at = NOW(),
		    updated_by = COALESCE($2, updated_by)
		WHERE id = $1 AND is_active = false
	`
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	tag, err := db.Exec(ctx, query, id, updatedBy)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *Store) UpdatePassword(ctx context.Context, id uuid.UUID, hash []byte, updatedBy *uuid.UUID) error {
	return s.updatePassword(ctx, s.db, id, hash, updatedBy)
}

func (s *Store) UpdatePasswordTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, hash []byte, updatedBy *uuid.UUID) error {
	return s.updatePassword(ctx, tx, id, hash, updatedBy)
}

func (s *Store) updatePassword(ctx context.Context, db userDBTX, id uuid.UUID, hash []byte, updatedBy *uuid.UUID) error {
	query := `
		UPDATE users
		SET password = $2,
		    version = version + 1,
		    updated_at = NOW(),
		    updated_by = COALESCE($3, updated_by)
		WHERE id = $1
	`
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	tag, err := db.Exec(ctx, query, id, hash, updatedBy)
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

func (s *Store) VersionTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	var version int64
	err := tx.QueryRow(ctx, `SELECT version FROM users WHERE id = $1`, id).Scan(&version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, storage.ErrNotFound
		}
		return 0, err
	}
	return version, nil
}

// UpdateRole sets users.role_id, bumps version + token_version
// so existing access JWTs die immediately after a privilege change.
func (s *Store) UpdateRole(ctx context.Context, userID uuid.UUID, roleID int64, updatedBy *uuid.UUID) (*User, error) {
	query := `
		UPDATE users
		SET role_id = $2,
		    version = version + 1,
		    token_version = token_version + 1,
		    updated_at = NOW(),
		    updated_by = COALESCE($3, updated_by)
		WHERE id = $1
		RETURNING id
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	var id uuid.UUID
	err := s.db.QueryRow(ctx, query, userID, roleID, updatedBy).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, storage.MapError(err)
	}
	return s.GetByID(ctx, id)
}

func (s *Store) List(ctx context.Context, q ListQuery) (*query.ListResult[*User], error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	where := []string{"TRUE"}
	args := []any{}
	argN := 1

	if q.Search != "" {
		where = append(where, fmt.Sprintf(
			`(u.email ILIKE '%%' || $%d || '%%'
			  OR u.username ILIKE '%%' || $%d || '%%'
			  OR u.reference_id ILIKE '%%' || $%d || '%%')`,
			argN, argN, argN,
		))
		args = append(args, q.Search)
		argN++
	}

	if q.Role != "" {
		where = append(where, fmt.Sprintf("r.name = $%d", argN))
		args = append(args, q.Role)
		argN++
	}

	if q.IsActive != nil {
		where = append(where, fmt.Sprintf("COALESCE(u.is_active, false) = $%d", argN))
		args = append(args, *q.IsActive)
		argN++
	}

	var cursorPos *query.Cursor
	if q.UsingCursor() {
		c, err := query.DecodeCursor(q.Cursor)
		if err != nil {
			return nil, err
		}
		cursorPos = &c
		if q.Sort.Order == "asc" {
			where = append(where, fmt.Sprintf("(u.created_at, u.id) > ($%d::timestamptz, $%d::uuid)", argN, argN+1))
		} else {
			where = append(where, fmt.Sprintf("(u.created_at, u.id) < ($%d::timestamptz, $%d::uuid)", argN, argN+1))
		}
		args = append(args, cursorPos.CreatedAt, cursorPos.ID)
		argN += 2
	}

	whereSQL := strings.Join(where, " AND ")
	fromSQL := `FROM users u JOIN roles r ON r.id = u.role_id WHERE ` + whereSQL
	result := &query.ListResult[*User]{Items: make([]*User, 0)}

	if !q.UsingCursor() {
		countQuery := `SELECT COUNT(*) ` + fromSQL
		if err := s.db.QueryRow(ctx, countQuery, args...).Scan(&result.Total); err != nil {
			return nil, err
		}
	}

	listArgs := append([]any{}, args...)
	var listQuery string
	if q.UsingCursor() {
		dir := "DESC"
		if q.Sort.Order == "asc" {
			dir = "ASC"
		}
		listArgs = append(listArgs, q.Limit)
		listQuery = fmt.Sprintf(`
			SELECT %s
			%s
			ORDER BY u.created_at %s, u.id %s
			LIMIT $%d
		`, userColumns, fromSQL, dir, dir, argN)
	} else {
		listArgs = append(listArgs, q.Limit, q.Offset)
		listQuery = fmt.Sprintf(`
			SELECT %s
			%s
			ORDER BY %s
			LIMIT $%d OFFSET $%d
		`, userColumns, fromSQL, q.Sort.Clause(), argN, argN+1)
	}

	rows, err := s.db.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		result.Items = append(result.Items, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if q.UsingCursor() && len(result.Items) == q.Limit {
		last := result.Items[len(result.Items)-1]
		result.NextCursor = query.EncodeCursor(last.CreatedAt, last.ID)
	}

	return result, nil
}

type userScanner interface {
	Scan(dest ...any) error
}

func scanUser(row userScanner) (*User, error) {
	var u User
	err := row.Scan(
		&u.ID,
		&u.ReferenceID,
		&u.Email,
		&u.Username,
		&u.Password.hash,
		&u.IsActive,
		&u.IsVisible,
		&u.Metadata,
		&u.RoleID,
		&u.Version,
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
		return nil, err
	}
	u.Metadata = metaOrEmpty(u.Metadata)
	return &u, nil
}
