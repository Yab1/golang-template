package user

import (
	"context"
	"errors"
	"time"

	"github.com/Yab1/golang-template/internal/platform/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errRefreshReuse = errors.New("refresh token reuse detected")

type RefreshSession struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	JTI       uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type RefreshStore struct {
	db *pgxpool.Pool
}

func NewRefreshStore(db *pgxpool.Pool) *RefreshStore {
	return &RefreshStore{db: db}
}

func (s *RefreshStore) Insert(ctx context.Context, sess *RefreshSession) error {
	query := `
		INSERT INTO refresh_tokens (user_id, jti, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	var createdAt time.Time
	return s.db.QueryRow(ctx, query, sess.UserID, sess.JTI, sess.ExpiresAt).Scan(&sess.ID, &createdAt)
}

func (s *RefreshStore) GetByJTI(ctx context.Context, jti uuid.UUID) (*RefreshSession, error) {
	query := `
		SELECT id, user_id, jti, expires_at, revoked_at
		FROM refresh_tokens
		WHERE jti = $1
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	var sess RefreshSession
	err := s.db.QueryRow(ctx, query, jti).Scan(
		&sess.ID,
		&sess.UserID,
		&sess.JTI,
		&sess.ExpiresAt,
		&sess.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	return &sess, nil
}

func (s *RefreshStore) Rotate(ctx context.Context, oldJTI, newJTI uuid.UUID, userID uuid.UUID, expiresAt time.Time) error {
	return storage.WithTx(s.db, ctx, func(tx pgx.Tx) error {
		ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
		defer cancel()

		var revokedAt *time.Time
		err := tx.QueryRow(ctx, `
			SELECT revoked_at FROM refresh_tokens WHERE jti = $1 FOR UPDATE
		`, oldJTI).Scan(&revokedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return storage.ErrNotFound
			}
			return err
		}
		if revokedAt != nil {
			return errRefreshReuse
		}

		var newID uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO refresh_tokens (user_id, jti, expires_at)
			VALUES ($1, $2, $3)
			RETURNING id
		`, userID, newJTI, expiresAt).Scan(&newID); err != nil {
			return err
		}

		tag, err := tx.Exec(ctx, `
			UPDATE refresh_tokens
			SET revoked_at = NOW(), replaced_by = $2
			WHERE jti = $1 AND revoked_at IS NULL
		`, oldJTI, newID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errRefreshReuse
		}
		return nil
	})
}

func (s *RefreshStore) Revoke(ctx context.Context, jti uuid.UUID) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE jti = $1 AND revoked_at IS NULL
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	_, err := s.db.Exec(ctx, query, jti)
	return err
}

func (s *RefreshStore) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE user_id = $1 AND revoked_at IS NULL
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	_, err := s.db.Exec(ctx, query, userID)
	return err
}
