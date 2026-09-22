package outbox

import (
	"context"
	"testing"

	"github.com/Yab1/golang-template/internal/platform/event"
)

func TestEnqueueRejectsInvalidEvent(t *testing.T) {
	t.Parallel()

	err := NewStore(nil).Enqueue(context.Background(), nil, "topic", "agg", "post", event.Event{}, nil)
	if err == nil {
		t.Fatal("expected validation error before any database write")
	}
}
