package file

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Object struct {
	Key         string          `json:"key"`
	OwnerID     uuid.UUID       `json:"owner_id"`
	ContentType string          `json:"content_type"`
	Size        int64           `json:"size"`
	Driver      string          `json:"driver"`
	Version     int64           `json:"version"`
	IsVisible   bool            `json:"is_visible"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedBy   *uuid.UUID      `json:"created_by,omitempty"`
	UpdatedBy   *uuid.UUID      `json:"updated_by,omitempty"`
	DeletedBy   *uuid.UUID      `json:"-"`
	DeletedAt   *time.Time      `json:"-"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
