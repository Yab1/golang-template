package user

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Yab1/golang-template/internal/platform/event"
)

func TestUserRegisteredPayloadOmitsSecrets(t *testing.T) {
	t.Parallel()

	e, err := event.New(event.NewParams{
		Source:           "/services/api",
		Type:             event.TypeUserRegistered,
		Subject:          "urn:user:20000000-0000-4000-8000-000000000001",
		DataSchema:       event.SchemaUserRegistered,
		AggregateVersion: 1,
		Data: map[string]any{
			"user_id":      uuid.MustParse("20000000-0000-4000-8000-000000000001"),
			"reference_id": "GTL-USR-A7K2M",
			"is_active":    false,
			"is_visible":   true,
			"version":      1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(e.Data)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))
	for _, leaked := range []string{"email", "password", "token", "role"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("registered payload leaked %q: %s", leaked, body)
		}
	}
}
