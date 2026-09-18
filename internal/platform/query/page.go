package query

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// Page supports offset pagination and optional opaque cursor (keyset).
// If Cursor is set, Offset is ignored by list handlers that support keyset.
type Page struct {
	Limit  int    `json:"limit" validate:"gte=1,lte=100"`
	Offset int    `json:"offset" validate:"gte=0"`
	Cursor string `json:"cursor" validate:"omitempty,max=512"`
}

func NewPage() Page {
	return Page{
		Limit:  DefaultLimit,
		Offset: 0,
	}
}

func ParsePage(r *http.Request) (Page, error) {
	page := NewPage()
	qs := r.URL.Query()

	if limit := qs.Get("limit"); limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil {
			return page, fmt.Errorf("invalid limit")
		}
		if n < 1 || n > MaxLimit {
			return page, fmt.Errorf("limit must be between 1 and %d", MaxLimit)
		}
		page.Limit = n
	}

	if offset := qs.Get("offset"); offset != "" {
		n, err := strconv.Atoi(offset)
		if err != nil {
			return page, fmt.Errorf("invalid offset")
		}
		if n < 0 {
			return page, fmt.Errorf("offset must be >= 0")
		}
		page.Offset = n
	}

	if cursor := qs.Get("cursor"); cursor != "" {
		page.Cursor = cursor
		if _, err := DecodeCursor(cursor); err != nil {
			return page, fmt.Errorf("invalid cursor")
		}
	}

	return page, nil
}

func (p Page) UsingCursor() bool {
	return p.Cursor != ""
}

// Cursor is a keyset position on (created_at, id).
type Cursor struct {
	CreatedAt time.Time `json:"t"`
	ID        uuid.UUID `json:"i"`
}

func EncodeCursor(createdAt time.Time, id uuid.UUID) string {
	raw, err := json.Marshal(Cursor{CreatedAt: createdAt.UTC(), ID: id})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func DecodeCursor(s string) (Cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, err
	}
	var c Cursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return Cursor{}, err
	}
	if c.ID == uuid.Nil || c.CreatedAt.IsZero() {
		return Cursor{}, fmt.Errorf("incomplete cursor")
	}
	return c, nil
}
