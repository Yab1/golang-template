package post

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Yab1/golang-template/internal/platform/event"
)

func TestPostCreatedPayloadOmitsContent(t *testing.T) {
	t.Parallel()

	e, err := event.New(event.NewParams{
		Source:           "/services/api",
		Type:             event.TypePostCreated,
		Subject:          "urn:post:20000000-0000-4000-8000-000000000001",
		DataSchema:       event.SchemaPostCreated,
		AggregateVersion: 1,
		Data: map[string]any{
			"post_id":       uuid.MustParse("20000000-0000-4000-8000-000000000001"),
			"reference_id":  "GTL-PST-A7K2M",
			"owner_user_id": uuid.MustParse("20000000-0000-4000-8000-000000000002"),
			"is_visible":    true,
			"version":       1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(e.Data))
	for _, leaked := range []string{"title", "content", "tags", "metadata"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("created payload leaked %q: %s", leaked, body)
		}
	}
	var payload map[string]any
	if err := json.Unmarshal(e.Data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["owner_user_id"] == nil {
		t.Fatal("owner_user_id required")
	}
}
