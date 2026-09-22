package user

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Yab1/golang-template/internal/platform/event"
	"github.com/Yab1/golang-template/internal/platform/stamp"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

func (m *Module) create(ctx context.Context, r *http.Request, u *User) error {
	if m.events == nil {
		return m.users.Create(ctx, u)
	}
	return storage.WithTx(m.db, ctx, func(tx pgx.Tx) error {
		if err := m.users.CreateTx(ctx, tx, u); err != nil {
			return err
		}
		e, err := m.userEvent(r, u.ID, u.Version, event.TypeUserRegistered, event.SchemaUserRegistered, map[string]any{
			"user_id":      u.ID,
			"reference_id": u.ReferenceID,
			"is_active":    u.IsActive,
			"is_visible":   u.IsVisible,
			"version":      u.Version,
		})
		if err != nil {
			return err
		}
		return m.events.Enqueue(ctx, tx, m.eventTopic, u.ID.String(), "user", e, userEventHeaders(r))
	})
}

func (m *Module) activate(ctx context.Context, r *http.Request, id uuid.UUID) error {
	actor := stamp.Ptr(id)
	if m.events == nil {
		_, err := m.users.Activate(ctx, id, actor)
		return err
	}
	return storage.WithTx(m.db, ctx, func(tx pgx.Tx) error {
		changed, err := m.users.ActivateTx(ctx, tx, id, actor)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		version, err := m.users.VersionTx(ctx, tx, id)
		if err != nil {
			return err
		}
		e, err := m.userEvent(r, id, version, event.TypeUserActivated, event.SchemaUserActivated, map[string]any{
			"user_id": id,
			"version": version,
		})
		if err != nil {
			return err
		}
		return m.events.Enqueue(ctx, tx, m.eventTopic, id.String(), "user", e, userEventHeaders(r))
	})
}

func (m *Module) updatePassword(
	ctx context.Context,
	r *http.Request,
	id uuid.UUID,
	hash []byte,
	reason string,
) error {
	actor := stamp.Ptr(id)
	if m.events == nil {
		return m.users.UpdatePassword(ctx, id, hash, actor)
	}
	return storage.WithTx(m.db, ctx, func(tx pgx.Tx) error {
		if err := m.users.UpdatePasswordTx(ctx, tx, id, hash, actor); err != nil {
			return err
		}
		version, err := m.users.VersionTx(ctx, tx, id)
		if err != nil {
			return err
		}
		e, err := m.userEvent(r, id, version, event.TypeUserPasswordChanged, event.SchemaUserPasswordChanged, map[string]any{
			"user_id":          id,
			"change_reason":    reason,
			"sessions_revoked": true,
			"version":          version,
		})
		if err != nil {
			return err
		}
		return m.events.Enqueue(ctx, tx, m.eventTopic, id.String(), "user", e, userEventHeaders(r))
	})
}

func (m *Module) userEvent(r *http.Request, id uuid.UUID, version int64, eventType, schemaURI string, data any) (event.Event, error) {
	return event.New(event.NewParams{
		Source:           m.eventSource,
		Type:             eventType,
		Subject:          "urn:user:" + id.String(),
		DataSchema:       schemaURI,
		CorrelationID:    middleware.GetReqID(r.Context()),
		TraceParent:      r.Header.Get("traceparent"),
		AggregateVersion: version,
		Data:             data,
	})
}

func userEventHeaders(r *http.Request) map[string]string {
	headers := map[string]string{}
	if requestID := middleware.GetReqID(r.Context()); requestID != "" {
		headers["correlation-id"] = requestID
	}
	if traceParent := r.Header.Get("traceparent"); traceParent != "" {
		headers["traceparent"] = traceParent
	}
	return headers
}
