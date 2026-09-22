package stamp

import "github.com/google/uuid"

// Ptr returns a pointer to id for nullable actor columns.
func Ptr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
