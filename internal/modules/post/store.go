package post

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yab1/golang-template/internal/platform/refid"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

const RefCode = "PST"

type Store struct {
	db   *pgxpool.Pool
	refs *refid.Generator
}

func NewStore(db *pgxpool.Pool, refs *refid.Generator) *Store {
	return &Store{db: db, refs: refs}
}

func (s *Store) Create(ctx context.Context, p *Post) error {
	query := `
		INSERT INTO posts (user_id, title, content, tags, reference_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, version, created_at, updated_at
	`

	if p.Tags == nil {
		p.Tags = []string{}
	}

	var lastErr error
	for i := 0; i < refid.MaxTries(); i++ {
		ref, err := s.refs.Next(RefCode)
		if err != nil {
			return err
		}
		p.ReferenceID = ref

		ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
		err = s.db.QueryRow(
			ctx,
			query,
			p.UserID,
			p.Title,
			p.Content,
			p.Tags,
			p.ReferenceID,
		).Scan(
			&p.ID,
			&p.Version,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		cancel()
		if err == nil {
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

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (*Post, error) {
	return s.getPost(ctx, `
		SELECT id, reference_id, user_id, title, content, tags, version, created_at, updated_at
		FROM posts
		WHERE id = $1
	`, id)
}

func (s *Store) GetByReferenceID(ctx context.Context, ref string) (*Post, error) {
	return s.getPost(ctx, `
		SELECT id, reference_id, user_id, title, content, tags, version, created_at, updated_at
		FROM posts
		WHERE reference_id = $1
	`, strings.ToUpper(ref))
}

func (s *Store) getPost(ctx context.Context, query string, arg any) (*Post, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	var p Post
	err := s.db.QueryRow(ctx, query, arg).Scan(
		&p.ID,
		&p.ReferenceID,
		&p.UserID,
		&p.Title,
		&p.Content,
		&p.Tags,
		&p.Version,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}

	if p.Tags == nil {
		p.Tags = []string{}
	}

	return &p, nil
}

func (s *Store) Update(ctx context.Context, p *Post) error {
	query := `
		UPDATE posts
		SET title = $1,
		    content = $2,
		    tags = $3,
		    version = version + 1,
		    updated_at = NOW()
		WHERE id = $4 AND version = $5
		RETURNING version, updated_at
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	if p.Tags == nil {
		p.Tags = []string{}
	}

	err := s.db.QueryRow(
		ctx,
		query,
		p.Title,
		p.Content,
		p.Tags,
		p.ID,
		p.Version,
	).Scan(
		&p.Version,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return storage.ErrVersionMismatch
		}
		return err
	}

	return nil
}

func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM posts WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	tag, err := s.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}

	return nil
}

func (s *Store) List(ctx context.Context, q ListQuery) (*ListResult, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	where := []string{"TRUE"}
	args := []any{}
	argN := 1

	if q.Search != "" {
		where = append(where, fmt.Sprintf("(title ILIKE '%%' || $%d || '%%' OR content ILIKE '%%' || $%d || '%%')", argN, argN))
		args = append(args, q.Search)
		argN++
	}

	if len(q.Tags) > 0 {
		where = append(where, fmt.Sprintf("tags @> $%d", argN))
		args = append(args, q.Tags)
		argN++
	}

	if q.UserID != nil {
		where = append(where, fmt.Sprintf("user_id = $%d", argN))
		args = append(args, *q.UserID)
		argN++
	}

	whereSQL := strings.Join(where, " AND ")

	countQuery := `SELECT COUNT(*) FROM posts WHERE ` + whereSQL
	var total int64
	if err := s.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, q.Limit, q.Offset)
	limitPh := argN
	offsetPh := argN + 1

	listQuery := fmt.Sprintf(`
		SELECT id, reference_id, user_id, title, content, tags, version, created_at, updated_at
		FROM posts
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereSQL, q.Sort.Clause(), limitPh, offsetPh)

	rows, err := s.db.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*Post, 0)
	for rows.Next() {
		var p Post
		if err := rows.Scan(
			&p.ID,
			&p.ReferenceID,
			&p.UserID,
			&p.Title,
			&p.Content,
			&p.Tags,
			&p.Version,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if p.Tags == nil {
			p.Tags = []string{}
		}
		items = append(items, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &ListResult{
		Items: items,
		Total: total,
	}, nil
}
