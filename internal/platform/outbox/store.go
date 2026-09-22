package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yab1/golang-template/internal/platform/event"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Store struct {
	db *pgxpool.Pool
}

type Record struct {
	ID               uuid.UUID
	Topic            string
	Key              string
	EventType        string
	AggregateType    string
	AggregateID      string
	AggregateVersion int64
	SchemaURI        string
	Payload          []byte
	Headers          map[string]string
	OccurredAt       time.Time
	Attempts         int
	MaxAttempts      int
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) Enqueue(ctx context.Context, q DBTX, topic, key, aggregateType string, e event.Event, headers map[string]string) error {
	if q == nil {
		q = s.db
	}
	if err := e.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal outbox event: %w", err)
	}
	rawHeaders, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("marshal outbox headers: %w", err)
	}
	query := `
		INSERT INTO event_outbox (
			id, topic, event_key, event_type, aggregate_type, aggregate_id,
			aggregate_version, schema_uri, payload, headers, occurred_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11)
	`
	_, err = q.Exec(
		ctx,
		query,
		e.ID,
		topic,
		key,
		e.Type,
		aggregateType,
		key,
		e.AggregateVersion,
		e.DataSchema,
		payload,
		rawHeaders,
		e.Time,
	)
	if err != nil {
		return fmt.Errorf("enqueue event: %w", err)
	}
	return nil
}

func (s *Store) Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]Record, error) {
	if limit < 1 {
		limit = 100
	}
	cutoff := time.Now().UTC().Add(-lease)
	query := `
		WITH candidates AS (
			SELECT id
			FROM event_outbox
			WHERE published_at IS NULL
			  AND attempts < max_attempts
			  AND available_at <= NOW()
			  AND (locked_at IS NULL OR locked_at < $3)
			ORDER BY available_at, occurred_at
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE event_outbox AS o
		SET locked_at = NOW(),
		    locked_by = $2,
		    attempts = attempts + 1
		FROM candidates
		WHERE o.id = candidates.id
		RETURNING
			o.id, o.topic, o.event_key, o.event_type, o.aggregate_type,
			o.aggregate_id, o.aggregate_version, o.schema_uri, o.payload,
			o.headers, o.occurred_at, o.attempts, o.max_attempts
	`
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, query, limit, workerID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]Record, 0, limit)
	for rows.Next() {
		var record Record
		var headers []byte
		if err := rows.Scan(
			&record.ID,
			&record.Topic,
			&record.Key,
			&record.EventType,
			&record.AggregateType,
			&record.AggregateID,
			&record.AggregateVersion,
			&record.SchemaURI,
			&record.Payload,
			&headers,
			&record.OccurredAt,
			&record.Attempts,
			&record.MaxAttempts,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(headers, &record.Headers); err != nil {
			return nil, fmt.Errorf("decode outbox headers: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) MarkPublished(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE event_outbox
		SET published_at = NOW(), locked_at = NULL, locked_by = NULL, last_error = NULL
		WHERE id = $1
	`
	tag, err := s.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *Store) MarkFailed(ctx context.Context, id uuid.UUID, cause error, delay time.Duration) error {
	if cause == nil {
		return errors.New("failure cause is required")
	}
	query := `
		UPDATE event_outbox
		SET available_at = $2, locked_at = NULL, locked_by = NULL, last_error = $3
		WHERE id = $1
	`
	_, err := s.db.Exec(ctx, query, id, time.Now().UTC().Add(delay), cause.Error())
	return err
}

func (s *Store) Backlog(ctx context.Context) (count int64, oldest time.Duration, err error) {
	query := `
		SELECT COUNT(*), COALESCE(EXTRACT(EPOCH FROM (NOW() - MIN(occurred_at))), 0)
		FROM event_outbox
		WHERE published_at IS NULL
	`
	var seconds float64
	if err := s.db.QueryRow(ctx, query).Scan(&count, &seconds); err != nil {
		return 0, 0, err
	}
	return count, time.Duration(seconds * float64(time.Second)), nil
}
