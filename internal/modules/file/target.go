package file

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrTarget = errors.New("unknown file target")
	ErrSlot   = errors.New("resource, field, and id are required")
)

type Target struct {
	Class    string
	MaxBytes int64
	Types    []string
	Locate   func(ctx context.Context, id string) (uuid.UUID, error)
	Bind     func(ctx context.Context, r *http.Request, id string, object *Object, body []byte, tx pgx.Tx) (string, error)
}

type CallError struct {
	Status int
	Err    error
}

func (e *CallError) Error() string {
	if e == nil || e.Err == nil {
		return "file target failed"
	}
	return e.Err.Error()
}

func (e *CallError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func Status(status int, err error) error {
	if err == nil {
		return nil
	}
	return &CallError{Status: status, Err: err}
}

func (m *Module) Register(resource, field string, target Target) {
	if m.targets == nil {
		m.targets = map[string]Target{}
	}
	m.targets[targetKey(resource, field)] = target
}

func (m *Module) target(resource, field string) (Target, bool) {
	if m == nil || m.targets == nil {
		return Target{}, false
	}
	target, ok := m.targets[targetKey(resource, field)]
	return target, ok
}

func targetKey(resource, field string) string {
	return strings.ToLower(strings.TrimSpace(resource)) + "\x00" + strings.ToLower(strings.TrimSpace(field))
}
