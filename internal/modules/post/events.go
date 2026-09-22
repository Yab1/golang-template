package post

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"github.com/Yab1/golang-template/internal/platform/event"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

func (m *Module) create(ctx context.Context, r *http.Request, p *Post) error {
	if m.events == nil {
		return m.posts.Create(ctx, p)
	}
	return storage.WithTx(m.db, ctx, func(tx pgx.Tx) error {
		if err := m.posts.CreateTx(ctx, tx, p); err != nil {
			return err
		}
		e, err := m.postEvent(r, p, event.TypePostCreated, event.SchemaPostCreated, map[string]any{
			"post_id":       p.ID,
			"reference_id":  p.ReferenceID,
			"owner_user_id": p.UserID,
			"is_visible":    p.IsVisible,
			"version":       p.Version,
		})
		if err != nil {
			return err
		}
		return m.events.Enqueue(ctx, tx, m.eventTopic, p.ID.String(), "post", e, eventHeaders(r))
	})
}

func (m *Module) update(ctx context.Context, r *http.Request, p *Post, previousVisible bool, changed []string) error {
	if m.events == nil {
		return m.posts.Update(ctx, p)
	}
	return storage.WithTx(m.db, ctx, func(tx pgx.Tx) error {
		if err := m.posts.UpdateTx(ctx, tx, p); err != nil {
			return err
		}
		eventType := event.TypePostUpdated
		schemaURI := event.SchemaPostUpdated
		data := map[string]any{
			"post_id":        p.ID,
			"changed_fields": changed,
			"version":        p.Version,
		}
		if previousVisible != p.IsVisible {
			eventType = event.TypePostVisibilityChanged
			schemaURI = event.SchemaPostVisibilityChanged
			data = map[string]any{
				"post_id":             p.ID,
				"previous_visibility": previousVisible,
				"is_visible":          p.IsVisible,
				"version":             p.Version,
			}
		} else if len(changed) == 0 {
			return nil
		}
		e, err := m.postEvent(r, p, eventType, schemaURI, data)
		if err != nil {
			return err
		}
		return m.events.Enqueue(ctx, tx, m.eventTopic, p.ID.String(), "post", e, eventHeaders(r))
	})
}

func (m *Module) delete(ctx context.Context, r *http.Request, p *Post) error {
	if m.events == nil {
		return m.posts.SoftDelete(ctx, p)
	}
	return storage.WithTx(m.db, ctx, func(tx pgx.Tx) error {
		if err := m.posts.SoftDeleteTx(ctx, tx, p); err != nil {
			return err
		}
		e, err := m.postEvent(r, p, event.TypePostDeleted, event.SchemaPostDeleted, map[string]any{
			"post_id": p.ID,
			"version": p.Version,
		})
		if err != nil {
			return err
		}
		return m.events.Enqueue(ctx, tx, m.eventTopic, p.ID.String(), "post", e, eventHeaders(r))
	})
}

func (m *Module) postEvent(r *http.Request, p *Post, eventType, schemaURI string, data any) (event.Event, error) {
	return event.New(event.NewParams{
		Source:           m.eventSource,
		Type:             eventType,
		Subject:          "urn:post:" + p.ID.String(),
		DataSchema:       schemaURI,
		CorrelationID:    middleware.GetReqID(r.Context()),
		TraceParent:      r.Header.Get("traceparent"),
		AggregateVersion: int64(max(p.Version, 1)),
		Data:             data,
	})
}

func eventHeaders(r *http.Request) map[string]string {
	headers := map[string]string{}
	if requestID := middleware.GetReqID(r.Context()); requestID != "" {
		headers["correlation-id"] = requestID
	}
	if traceParent := r.Header.Get("traceparent"); traceParent != "" {
		headers["traceparent"] = traceParent
	}
	return headers
}
