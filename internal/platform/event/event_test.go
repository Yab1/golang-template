package event

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNewRoundTrip(t *testing.T) {
	t.Parallel()

	original, err := New(NewParams{
		Source:           "test/api",
		Type:             TypePostCreated,
		Subject:          "post/123",
		DataSchema:       SchemaPostCreated,
		AggregateVersion: 1,
		Data:             map[string]any{"post_id": "123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ID != original.ID {
		t.Fatalf("event id = %s, want %s", decoded.ID, original.ID)
	}
}

func TestPermanent(t *testing.T) {
	t.Parallel()

	base := errors.New("invalid payload")
	wrapped := Permanent(base)
	if !IsPermanent(wrapped) {
		t.Fatal("expected permanent error")
	}
	if !errors.Is(wrapped, base) {
		t.Fatal("expected wrapped cause")
	}
}

func TestTopics(t *testing.T) {
	t.Parallel()

	source := Topic("production.app", "content", "post")
	if source != "production.app.content.post.events.v1" {
		t.Fatalf("source topic = %q", source)
	}
	if got := RetryTopic(source, "search"); got != source+".search.retry.v1" {
		t.Fatalf("retry topic = %q", got)
	}
	if got := DLQTopic(source, "search"); got != source+".search.dlq.v1" {
		t.Fatalf("DLQ topic = %q", got)
	}
	files := Topic("dev.app", "files", "file")
	if files != "dev.app.files.file.events.v1" {
		t.Fatalf("file topic = %q", files)
	}
}

func TestNewReplacesNonUUIDCorrelationID(t *testing.T) {
	t.Parallel()

	e, err := New(NewParams{
		Source:           "/services/api",
		Type:             TypePostCreated,
		Subject:          "urn:post:20000000-0000-4000-8000-000000000001",
		DataSchema:       SchemaPostCreated,
		CorrelationID:    "chi-request-id",
		AggregateVersion: 1,
		Data:             map[string]any{"post_id": "20000000-0000-4000-8000-000000000001"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if e.CorrelationID == "chi-request-id" {
		t.Fatal("correlation id must be a UUID")
	}
}

func TestIsStaleRejectsOlderAggregateVersions(t *testing.T) {
	t.Parallel()
	if !IsStale(1, 2) {
		t.Fatal("older version must be stale")
	}
	if IsStale(2, 1) {
		t.Fatal("newer version must apply")
	}
	if IsStale(2, 2) {
		t.Fatal("equal version is not stale")
	}
	if IsStale(1, 0) {
		t.Fatal("unseen aggregate is not stale")
	}
}
