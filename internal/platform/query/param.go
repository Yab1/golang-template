package query

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// OptionalString returns trimmed query value or "".
func OptionalString(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

// ParseCSV splits a comma-separated query param into trimmed non-empty parts.
// Missing or empty key → nil slice (fine to range / store as empty).
func ParseCSV(r *http.Request, key string) []string {
	raw := OptionalString(r, key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ParseUUID returns nil if the key is absent/empty.
func ParseUUID(r *http.Request, key string) (*uuid.UUID, error) {
	raw := OptionalString(r, key)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s", key)
	}
	return &id, nil
}

// ParseBool returns (false, false) if absent; (value, true) if present.
func ParseBool(r *http.Request, key string) (value bool, set bool, err error) {
	raw := OptionalString(r, key)
	if raw == "" {
		return false, false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, true, fmt.Errorf("invalid %s", key)
	}
	return v, true, nil
}

// ParseInt returns (0, false, nil) if absent.
func ParseInt(r *http.Request, key string) (value int, set bool, err error) {
	raw := OptionalString(r, key)
	if raw == "" {
		return 0, false, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, true, fmt.Errorf("invalid %s", key)
	}
	return v, true, nil
}
