package notification

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	ChannelEmail    = "email"
	ChannelSMS      = "sms"
	ChannelPush     = "push"
	ChannelInApp    = "in_app"
	ChannelTelegram = "telegram"

	CategoryWelcome       = "welcome"
	CategoryAlert         = "alert"
	CategoryStatus        = "status"
	CategoryTransactional = "transactional"

	FormatText     = "text"
	FormatHTML     = "html"
	FormatMarkdown = "markdown"

	StatusPending = "pending"
	StatusSending = "sending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"

	RefCodeTemplate = "NTM"
	RefCodeDelivery = "NTF"
)

var Channels = []string{ChannelEmail, ChannelSMS, ChannelPush, ChannelInApp, ChannelTelegram}

type Template struct {
	ID          uuid.UUID       `json:"id"`
	ReferenceID string          `json:"reference_id"`
	Channel     string          `json:"channel"`
	Category    string          `json:"category"`
	Locale      string          `json:"locale"`
	Subject     string          `json:"subject"`
	Body        string          `json:"body"`
	Format      string          `json:"format"`
	IsActive    bool            `json:"is_active"`
	Version     int             `json:"version"`
	IsVisible   bool            `json:"is_visible"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	CreatedBy   *uuid.UUID      `json:"created_by,omitempty"`
	UpdatedBy   *uuid.UUID      `json:"updated_by,omitempty"`
	DeletedAt   *time.Time      `json:"-"`
	DeletedBy   *uuid.UUID      `json:"-"`
}

type Preference struct {
	ID          uuid.UUID       `json:"id"`
	UserID      uuid.UUID       `json:"user_id"`
	Channel     string          `json:"channel"`
	Category    string          `json:"category"`
	Enabled     bool            `json:"enabled"`
	Destination string          `json:"destination"`
	Version     int             `json:"version"`
	IsVisible   bool            `json:"is_visible"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	CreatedBy   *uuid.UUID      `json:"created_by,omitempty"`
	UpdatedBy   *uuid.UUID      `json:"updated_by,omitempty"`
	DeletedAt   *time.Time      `json:"-"`
	DeletedBy   *uuid.UUID      `json:"-"`
}

type Delivery struct {
	ID             uuid.UUID       `json:"id"`
	ReferenceID    string          `json:"reference_id"`
	UserID         uuid.UUID       `json:"user_id"`
	Channel        string          `json:"channel"`
	Category       string          `json:"category"`
	TemplateID     *uuid.UUID      `json:"template_id,omitempty"`
	Destination    string          `json:"destination"`
	Subject        string          `json:"subject"`
	Body           string          `json:"body"`
	Format         string          `json:"format"`
	Status         string          `json:"status"`
	Attempts       int             `json:"attempts"`
	MaxAttempts    int             `json:"max_attempts"`
	NextAttemptAt  time.Time       `json:"next_attempt_at"`
	LastError      string          `json:"last_error"`
	ProviderRef    string          `json:"provider_ref"`
	IdempotencyKey *string         `json:"idempotency_key,omitempty"`
	ReadAt         *time.Time      `json:"read_at,omitempty"`
	LockedUntil    *time.Time      `json:"-"`
	Version        int             `json:"version"`
	IsVisible      bool            `json:"is_visible"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	CreatedBy      *uuid.UUID      `json:"created_by,omitempty"`
	UpdatedBy      *uuid.UUID      `json:"updated_by,omitempty"`
	DeletedAt      *time.Time      `json:"-"`
	DeletedBy      *uuid.UUID      `json:"-"`
	AttemptLog     []Attempt       `json:"attempt_log,omitempty"`
}

type Attempt struct {
	ID         uuid.UUID `json:"id"`
	DeliveryID uuid.UUID `json:"delivery_id"`
	Attempt    int       `json:"attempt"`
	Status     string    `json:"status"`
	Error      string    `json:"error"`
	CreatedAt  time.Time `json:"created_at"`
}

type Account struct {
	Email  string
	Active bool
}

type Accounts interface {
	Account(ctx context.Context, id uuid.UUID) (Account, error)
}
