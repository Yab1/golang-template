package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yab1/golang-template/internal/platform/storage"
)

const (
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
)

type Entry struct {
	ActorID      *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   string
	RequestID    string
	IP           string
	Meta         map[string]any
}

type Logger interface {
	Log(ctx context.Context, e Entry) error
}

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) Log(ctx context.Context, e Entry) error {
	meta := e.Meta
	if meta == nil {
		meta = map[string]any{}
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, request_id, ip, meta)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
	`
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()

	_, err = s.db.Exec(ctx, query, e.ActorID, e.Action, e.ResourceType, e.ResourceID, nullIfEmpty(e.RequestID), nullIfEmpty(e.IP), string(raw))
	return err
}

type Nop struct{}

func NewNop() *Nop { return &Nop{} }

func (Nop) Log(context.Context, Entry) error { return nil }

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// EntryTime is a helper for callers that want a UTC stamp in meta.
func EntryTime() time.Time { return time.Now().UTC() }
