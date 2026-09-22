package post

import (
	"time"

	"github.com/google/uuid"
)

type Post struct {
	ID          uuid.UUID  `json:"id"`
	ReferenceID string     `json:"reference_id"`
	UserID      uuid.UUID  `json:"user_id"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	Tags        []string   `json:"tags"`
	Version     int        `json:"version"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"-"`
}
