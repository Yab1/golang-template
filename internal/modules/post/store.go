package post

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yab1/golang-template/internal/platform/query"
	"github.com/Yab1/golang-template/internal/platform/refid"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

const RefCode = "PST"

const activePosts = "deleted_at IS NULL"
const visiblePosts = activePosts + " AND is_visible"

var emptyMeta = json.RawMessage(`{}`)

const postColumns = `id, reference_id, user_id, title, content, tags, version, is_visible, metadata, created_at, updated_at, created_by, updated_by, deleted_at, deleted_by`

type Store struct {
	db   *pgxpool.Pool
	refs *refid.Generator
}

type querier interface {
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

func scanPost(sc interface{ Scan(dest ...any) error }, p *Post) error {
	err := sc.Scan(
		&p.ID,
		&p.ReferenceID,
		&p.UserID,
		&p.Title,
		&p.Content,
		&p.Tags,
		&p.Version,
		&p.IsVisible,
		&p.Metadata,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.CreatedBy,
		&p.UpdatedBy,
		&p.DeletedAt,
		&p.DeletedBy,
	)
	if err != nil {
		return err
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}
	p.Metadata = metaOrEmpty(p.Metadata)
	return nil
}

func (s *Store) Create(ctx context.Context, p *Post) error {
	return s.create(ctx, s.db, p)
}

func (s *Store) CreateTx(ctx context.Context, tx pgx.Tx, p *Post) error {
	return s.create(ctx, tx, p)
}

func (s *Store) create(ctx context.Context, db querier, p *Post) error {
	q := `
		INSERT INTO posts (user_id, title, content, tags, reference_id, is_visible, metadata, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING ` + postColumns

	if p.Tags == nil {
		p.Tags = []string{}
	}
	p.Metadata = metaOrEmpty(p.Metadata)

	var lastErr error
	for i := 0; i < refid.MaxTries(); i++ {
		ref, err := s.refs.Next(RefCode)
		if err != nil {
			return err
		}
		p.ReferenceID = ref

		ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
		err = scanPost(db.QueryRow(
			ctx,
			q,
			p.UserID,
			p.Title,
			p.Content,
			p.Tags,
			p.ReferenceID,
			p.IsVisible,
			p.Metadata,
			p.CreatedBy,
			p.UpdatedBy,
		), p)
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
		SELECT `+postColumns+`
		FROM posts
		WHERE id = $1 AND `+activePosts+`
	`, id)
}

func (s *Store) GetByReferenceID(ctx context.Context, ref string) (*Post, error) {
	return s.getPost(ctx, `
		SELECT `+postColumns+`
		FROM posts
		WHERE reference_id = $1 AND `+activePosts+`
	`, strings.ToUpper(ref))
}

func (s *Store) getPost(ctx context.Context, q string, arg any) (*Post, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	var p Post
	err := scanPost(s.db.QueryRow(ctx, q, arg), &p)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *Store) Update(ctx context.Context, p *Post) error {
	return s.update(ctx, s.db, p)
}

func (s *Store) UpdateTx(ctx context.Context, tx pgx.Tx, p *Post) error {
	return s.update(ctx, tx, p)
}

func (s *Store) update(ctx context.Context, db querier, p *Post) error {
	q := `
		UPDATE posts
		SET title = $1,
		    content = $2,
		    tags = $3,
		    is_visible = $4,
		    metadata = $5,
		    version = version + 1,
		    updated_at = NOW(),
		    updated_by = $8
		WHERE id = $6 AND version = $7 AND ` + activePosts + `
		RETURNING version, is_visible, metadata, updated_at, updated_by, deleted_at, deleted_by
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	if p.Tags == nil {
		p.Tags = []string{}
	}
	p.Metadata = metaOrEmpty(p.Metadata)

	err := db.QueryRow(
		ctx,
		q,
		p.Title,
		p.Content,
		p.Tags,
		p.IsVisible,
		p.Metadata,
		p.ID,
		p.Version,
		p.UpdatedBy,
	).Scan(
		&p.Version,
		&p.IsVisible,
		&p.Metadata,
		&p.UpdatedAt,
		&p.UpdatedBy,
		&p.DeletedAt,
		&p.DeletedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return storage.ErrVersionMismatch
		}
		return err
	}
	p.Metadata = metaOrEmpty(p.Metadata)

	return nil
}

func (s *Store) SoftDelete(ctx context.Context, p *Post) error {
	return s.softDelete(ctx, s.db, p)
}

func (s *Store) SoftDeleteTx(ctx context.Context, tx pgx.Tx, p *Post) error {
	return s.softDelete(ctx, tx, p)
}

func (s *Store) softDelete(ctx context.Context, db querier, p *Post) error {
	q := `
		UPDATE posts
		SET deleted_at = NOW(),
		    updated_at = NOW(),
		    deleted_by = $2,
		    updated_by = $2
		WHERE id = $1 AND ` + activePosts + `
		RETURNING updated_at, updated_by, deleted_at, deleted_by
	`

	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	err := db.QueryRow(ctx, q, p.ID, p.DeletedBy).Scan(
		&p.UpdatedAt,
		&p.UpdatedBy,
		&p.DeletedAt,
		&p.DeletedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return storage.ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Store) List(ctx context.Context, q ListQuery) (*query.ListResult[*Post], error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	where := []string{visiblePosts}
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

	var cursorPos *query.Cursor
	if q.UsingCursor() {
		c, err := query.DecodeCursor(q.Cursor)
		if err != nil {
			return nil, err
		}
		cursorPos = &c
		if q.Sort.Order == "asc" {
			where = append(where, fmt.Sprintf("(created_at, id) > ($%d::timestamptz, $%d::uuid)", argN, argN+1))
		} else {
			where = append(where, fmt.Sprintf("(created_at, id) < ($%d::timestamptz, $%d::uuid)", argN, argN+1))
		}
		args = append(args, cursorPos.CreatedAt, cursorPos.ID)
		argN += 2
	}

	whereSQL := strings.Join(where, " AND ")

	result := &query.ListResult[*Post]{Items: make([]*Post, 0)}

	if !q.UsingCursor() {
		countQuery := `SELECT COUNT(*) FROM posts WHERE ` + whereSQL
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
			FROM posts
			WHERE %s
			ORDER BY created_at %s, id %s
			LIMIT $%d
		`, postColumns, whereSQL, dir, dir, argN)
	} else {
		listArgs = append(listArgs, q.Limit, q.Offset)
		listQuery = fmt.Sprintf(`
			SELECT %s
			FROM posts
			WHERE %s
			ORDER BY %s
			LIMIT $%d OFFSET $%d
		`, postColumns, whereSQL, q.Sort.Clause(), argN, argN+1)
	}

	rows, err := s.db.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p Post
		if err := scanPost(rows, &p); err != nil {
			return nil, err
		}
		result.Items = append(result.Items, &p)
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
