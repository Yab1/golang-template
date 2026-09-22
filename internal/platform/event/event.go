package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const SpecVersion = "1.0"

type Event struct {
	SpecVersion      string          `json:"specversion"`
	ID               uuid.UUID       `json:"id"`
	Source           string          `json:"source"`
	Type             string          `json:"type"`
	Subject          string          `json:"subject"`
	Time             time.Time       `json:"time"`
	DataContentType  string          `json:"datacontenttype"`
	DataSchema       string          `json:"dataschema"`
	CorrelationID    string          `json:"correlationid,omitempty"`
	CausationID      string          `json:"causationid,omitempty"`
	TraceParent      string          `json:"traceparent,omitempty"`
	AggregateVersion int64           `json:"aggregateversion"`
	Data             json.RawMessage `json:"data"`
}

type NewParams struct {
	Source           string
	Type             string
	Subject          string
	DataSchema       string
	CorrelationID    string
	CausationID      string
	TraceParent      string
	AggregateVersion int64
	Data             any
}

func New(p NewParams) (Event, error) {
	raw, err := json.Marshal(p.Data)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event data: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Event{}, fmt.Errorf("create event id: %w", err)
	}
	e := Event{
		SpecVersion:      SpecVersion,
		ID:               id,
		Source:           p.Source,
		Type:             p.Type,
		Subject:          p.Subject,
		Time:             time.Now().UTC(),
		DataContentType:  "application/json",
		DataSchema:       p.DataSchema,
		CorrelationID:    p.CorrelationID,
		CausationID:      p.CausationID,
		TraceParent:      p.TraceParent,
		AggregateVersion: p.AggregateVersion,
		Data:             raw,
	}
	if _, err := uuid.Parse(e.CorrelationID); err != nil {
		e.CorrelationID = uuid.NewString()
	}
	if err := e.Validate(); err != nil {
		return Event{}, err
	}
	return e, nil
}

func (e Event) Validate() error {
	if e.SpecVersion != SpecVersion {
		return fmt.Errorf("unsupported specversion %q", e.SpecVersion)
	}
	if e.ID == uuid.Nil {
		return errors.New("event id is required")
	}
	if e.Source == "" {
		return errors.New("event source is required")
	}
	if e.Type == "" {
		return errors.New("event type is required")
	}
	if e.Subject == "" {
		return errors.New("event subject is required")
	}
	if e.Time.IsZero() {
		return errors.New("event time is required")
	}
	if e.DataContentType != "application/json" {
		return fmt.Errorf("unsupported content type %q", e.DataContentType)
	}
	if e.DataSchema == "" {
		return errors.New("event data schema is required")
	}
	if e.CorrelationID == "" {
		return errors.New("event correlation id is required")
	}
	if _, err := uuid.Parse(e.CorrelationID); err != nil {
		return errors.New("event correlation id must be a UUID")
	}
	if e.AggregateVersion < 1 {
		return errors.New("aggregate version must be positive")
	}
	if len(e.Data) == 0 || !json.Valid(e.Data) {
		return errors.New("event data must be valid JSON")
	}
	return nil
}

func IsStale(incoming, lastSeen int64) bool {
	return lastSeen > 0 && incoming < lastSeen
}

func Decode(raw []byte) (Event, error) {
	var e Event
	if err := json.Unmarshal(raw, &e); err != nil {
		return Event{}, fmt.Errorf("decode event: %w", err)
	}
	if err := e.Validate(); err != nil {
		return Event{}, err
	}
	return e, nil
}
