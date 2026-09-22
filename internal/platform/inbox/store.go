package inbox

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yab1/golang-template/internal/platform/event"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

var (
	ErrDuplicate = errors.New("event already processed")
	ErrStale     = errors.New("event is stale")
)

type Message struct {
	ConsumerName string
	EventID      uuid.UUID
	Topic        string
	Partition    int32
	Offset       int64
}

type Handler func(context.Context, pgx.Tx, event.Event) error

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) Process(ctx context.Context, message Message, e event.Event, handler Handler) error {
	return storage.WithTx(s.db, ctx, func(tx pgx.Tx) error {
		if err := rejectStale(ctx, tx, message.ConsumerName, e); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO event_inbox (
				consumer_name, event_id, topic, partition_id, offset_id
			)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (consumer_name, event_id) DO NOTHING
		`, message.ConsumerName, message.EventID, message.Topic, message.Partition, message.Offset)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrDuplicate
		}
		if err := advanceWatermark(ctx, tx, message.ConsumerName, e); err != nil {
			return err
		}
		if handler == nil {
			return nil
		}
		return handler(ctx, tx, e)
	})
}

func rejectStale(ctx context.Context, tx pgx.Tx, consumer string, e event.Event) error {
	var last int64
	err := tx.QueryRow(ctx, `
		SELECT last_version
		FROM event_inbox_watermarks
		WHERE consumer_name = $1 AND subject = $2
		FOR UPDATE
	`, consumer, e.Subject).Scan(&last)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if event.IsStale(e.AggregateVersion, last) {
		return ErrStale
	}
	return nil
}

func advanceWatermark(ctx context.Context, tx pgx.Tx, consumer string, e event.Event) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO event_inbox_watermarks (consumer_name, subject, last_version)
		VALUES ($1, $2, $3)
		ON CONFLICT (consumer_name, subject) DO UPDATE
		SET last_version = EXCLUDED.last_version,
		    updated_at = NOW()
		WHERE event_inbox_watermarks.last_version < EXCLUDED.last_version
	`, consumer, e.Subject, e.AggregateVersion)
	return err
}
