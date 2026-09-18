package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yab1/golang-template/internal/platform/storage"
)

const (
	PurposeEmailVerify   = "email_verify"
	PurposePasswordReset = "password_reset"
)

type UserToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Purpose   string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

type TokenStore struct {
	db *pgxpool.Pool
}

func NewTokenStore(db *pgxpool.Pool) *TokenStore {
	return &TokenStore{db: db}
}

// NewPlainToken returns a URL-safe token and its SHA-256 hash for storage.
func NewPlainToken() (plain string, hash []byte, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	plain = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plain))
	return plain, sum[:], nil
}

func HashToken(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}

func (s *TokenStore) Create(ctx context.Context, userID uuid.UUID, purpose string, hash []byte, expiresAt time.Time) error {
	query := `
		INSERT INTO user_tokens (user_id, token_hash, purpose, expires_at)
		VALUES ($1, $2, $3, $4)
	`
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	_, err := s.db.Exec(ctx, query, userID, hash, purpose, expiresAt)
	return err
}

// InvalidateOpen marks unused tokens of a purpose for the user as used (single active token).
func (s *TokenStore) InvalidateOpen(ctx context.Context, userID uuid.UUID, purpose string) error {
	query := `
		UPDATE user_tokens
		SET used_at = NOW()
		WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL
	`
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	_, err := s.db.Exec(ctx, query, userID, purpose)
	return err
}

func (s *TokenStore) Consume(ctx context.Context, plain, purpose string) (*UserToken, error) {
	hash := HashToken(plain)
	query := `
		UPDATE user_tokens
		SET used_at = NOW()
		WHERE token_hash = $1
		  AND purpose = $2
		  AND used_at IS NULL
		  AND expires_at > NOW()
		RETURNING id, user_id, purpose, expires_at, used_at
	`
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	var t UserToken
	err := s.db.QueryRow(ctx, query, hash, purpose).Scan(
		&t.ID,
		&t.UserID,
		&t.Purpose,
		&t.ExpiresAt,
		&t.UsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}
