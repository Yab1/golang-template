package notification

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Yab1/golang-template/internal/platform/event"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

// Request is one fan-out. Empty Channels means every channel.
type Request struct {
	UserID         uuid.UUID
	Category       string
	Channels       []string
	Data           map[string]any
	IdempotencyKey string
	Actor          *uuid.UUID
}

// Enqueue writes one delivery row per channel and returns them. A repeated idempotency key returns the first set.
func (m *Module) Enqueue(ctx context.Context, r *http.Request, req Request) ([]*Delivery, bool, error) {
	if !knownCategory(req.Category) {
		return nil, false, fmt.Errorf("unknown category %q", req.Category)
	}
	channels := req.Channels
	if len(channels) == 0 {
		channels = Channels
	}
	for _, channel := range channels {
		if !knownChannel(channel) {
			return nil, false, fmt.Errorf("unknown channel %q", channel)
		}
	}
	if m.accounts == nil {
		return nil, false, errors.New("notification accounts are not configured")
	}
	account, err := m.accounts.Account(ctx, req.UserID)
	if err != nil {
		return nil, false, err
	}

	var created []*Delivery
	var replay []*Delivery
	err = storage.WithTx(m.db, ctx, func(tx pgx.Tx) error {
		if req.IdempotencyKey != "" {
			existing, err := m.store.ListByIdempotency(ctx, tx, req.IdempotencyKey)
			if err != nil {
				return err
			}
			if len(existing) > 0 {
				replay = existing
				return nil
			}
		}
		for _, channel := range channels {
			row, err := m.build(ctx, tx, req, account, channel)
			if err != nil {
				return err
			}
			if err := m.store.CreateDeliveryTx(ctx, tx, row); err != nil {
				return err
			}
			if row.Status == StatusSkipped {
				if err := m.store.AddAttemptTx(ctx, tx, row.ID, 0, StatusSkipped, row.LastError); err != nil {
					return err
				}
			}
			if m.events != nil {
				ev, err := m.deliveryEvent(r, row)
				if err != nil {
					return err
				}
				if err := m.events.Enqueue(ctx, tx, m.topic(), row.ID.String(), "notification", ev, eventHeaders(r)); err != nil {
					return err
				}
			}
			created = append(created, row)
		}
		return nil
	})
	if isIdempotencyConflict(err) && req.IdempotencyKey != "" {
		rows, loadErr := m.store.ListByIdempotency(ctx, m.db, req.IdempotencyKey)
		return rows, true, loadErr
	}
	if err != nil {
		return nil, false, err
	}
	if replay != nil {
		return replay, true, nil
	}
	return created, false, nil
}

func (m *Module) build(ctx context.Context, tx pgx.Tx, req Request, account Account, channel string) (*Delivery, error) {
	on, why := m.channelGate(channel)
	optedOut := false
	dest := ""
	pref, err := m.store.Preference(ctx, tx, req.UserID, channel, req.Category)
	if err == nil {
		optedOut = !pref.Enabled
		dest = pref.Destination
	} else if !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	if channel == ChannelEmail && dest == "" {
		dest = account.Email
	}
	if channel == ChannelInApp {
		dest = req.UserID.String()
	}
	status, reason := gate(gateInput{
		active: account.Active, channelOn: on, channelWhy: why,
		optedOut: optedOut, needsAddress: needsAddress(channel), destination: dest,
	})

	tpl := fallbackTemplate(channel, req.Category)
	var templateID *uuid.UUID
	stored, err := m.store.ActiveTemplate(ctx, tx, channel, req.Category)
	if err == nil {
		tpl = *stored
		templateID = &stored.ID
	} else if !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	subject, err := renderBody(FormatText, tpl.Subject, req.Data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrTemplate, err.Error())
	}
	body, err := renderBody(tpl.Format, tpl.Body, req.Data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrTemplate, err.Error())
	}
	var key *string
	if req.IdempotencyKey != "" {
		key = &req.IdempotencyKey
	}
	return &Delivery{
		UserID: req.UserID, Channel: channel, Category: req.Category, TemplateID: templateID,
		Destination: dest, Subject: subject, Body: body, Format: tpl.Format,
		Status: status, MaxAttempts: m.cfg.MaxAttempts, LastError: reason, IdempotencyKey: key,
		IsVisible: true, CreatedBy: req.Actor, UpdatedBy: req.Actor,
	}, nil
}

func (m *Module) topic() string {
	return event.Topic(m.topicPrefix, "notify", "notification")
}

func eventHeaders(r *http.Request) map[string]string {
	h := map[string]string{}
	if r == nil {
		return h
	}
	if id := middleware.GetReqID(r.Context()); id != "" {
		h["correlation-id"] = id
	}
	if tp := r.Header.Get("traceparent"); tp != "" {
		h["traceparent"] = tp
	}
	return h
}

func (m *Module) deliveryEvent(r *http.Request, row *Delivery) (event.Event, error) {
	correlation := ""
	trace := ""
	if r != nil {
		correlation = middleware.GetReqID(r.Context())
		trace = r.Header.Get("traceparent")
	}
	return event.New(event.NewParams{
		Source: m.eventSource, Type: event.TypeNotificationQueued,
		Subject: "urn:notification:" + row.ID.String(), DataSchema: event.SchemaNotificationQueued,
		CorrelationID: correlation, TraceParent: trace,
		AggregateVersion: int64(max(row.Version, 1)),
		Data: map[string]any{
			"notification_id": row.ID,
			"reference_id":    row.ReferenceID,
			"user_id":         row.UserID,
			"channel":         row.Channel,
			"category":        row.Category,
			"status":          row.Status,
			"version":         max(row.Version, 1),
		},
	})
}
