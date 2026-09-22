package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/Yab1/golang-template/internal/platform/event"
	"github.com/Yab1/golang-template/internal/platform/inbox"
	"github.com/Yab1/golang-template/internal/platform/outbox"
)

func TestPermanentFailuresGoToDLQ(t *testing.T) {
	t.Parallel()

	sink := &capturePublisher{}
	consumer := testConsumer(t, sink, func(context.Context, inbox.Message, event.Event, inbox.Handler) error {
		return event.Permanent(errors.New("invalid schema"))
	})
	record := testRecord(t, "dev.content.post.events.v1", validEvent(t))
	if err := consumer.process(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if sink.topic != event.DLQTopic("dev.content.post.events.v1", "search") {
		t.Fatalf("topic = %q", sink.topic)
	}
	if headerValue(sink.record, headerErrorClass) != "permanent" {
		t.Fatalf("error class = %q", headerValue(sink.record, headerErrorClass))
	}
	if string(sink.record.Value) != string(record.Value) {
		t.Fatal("DLQ must preserve original event bytes")
	}
}

func TestTransientFailuresGoToRetry(t *testing.T) {
	t.Parallel()

	sink := &capturePublisher{}
	consumer := testConsumer(t, sink, func(context.Context, inbox.Message, event.Event, inbox.Handler) error {
		return errors.New("timeout")
	})
	record := testRecord(t, "dev.content.post.events.v1", validEvent(t))
	if err := consumer.process(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if sink.topic != event.RetryTopic("dev.content.post.events.v1", "search") {
		t.Fatalf("topic = %q", sink.topic)
	}
	if headerValue(sink.record, headerErrorClass) != "transient" {
		t.Fatalf("error class = %q", headerValue(sink.record, headerErrorClass))
	}
	if headerValue(sink.record, headerNotBefore) == "" {
		t.Fatal("retry must set not-before")
	}
}

func TestExhaustedTransientFailuresGoToDLQ(t *testing.T) {
	t.Parallel()

	sink := &capturePublisher{}
	consumer := testConsumer(t, sink, func(context.Context, inbox.Message, event.Event, inbox.Handler) error {
		return errors.New("timeout")
	})
	consumer.cfg.MaxAttempts = 4
	record := testRecord(t, "dev.content.post.events.v1", validEvent(t))
	record.Headers = append(record.Headers, kgo.RecordHeader{Key: headerAttempt, Value: []byte("3")})
	if err := consumer.process(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if sink.topic != event.DLQTopic("dev.content.post.events.v1", "search") {
		t.Fatalf("topic = %q", sink.topic)
	}
}

func TestDuplicateDeliveryIsSuccessfulNoOp(t *testing.T) {
	t.Parallel()

	sink := &capturePublisher{}
	calls := 0
	consumer := testConsumer(t, sink, func(context.Context, inbox.Message, event.Event, inbox.Handler) error {
		calls++
		if calls == 1 {
			return nil
		}
		return inbox.ErrDuplicate
	})
	record := testRecord(t, "dev.content.post.events.v1", validEvent(t))
	if err := consumer.process(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if err := consumer.process(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if sink.topic != "" {
		t.Fatalf("duplicate must not retry or DLQ, got %q", sink.topic)
	}
}

func TestStaleAggregateVersionIsSuccessfulNoOp(t *testing.T) {
	t.Parallel()

	sink := &capturePublisher{}
	consumer := testConsumer(t, sink, func(context.Context, inbox.Message, event.Event, inbox.Handler) error {
		return inbox.ErrStale
	})
	if err := consumer.process(context.Background(), testRecord(t, "dev.content.post.events.v1", validEvent(t))); err != nil {
		t.Fatal(err)
	}
	if sink.topic != "" {
		t.Fatalf("stale event must not retry or DLQ, got %q", sink.topic)
	}
}

func TestRelayPublishesThenMarks(t *testing.T) {
	t.Parallel()

	id := uuid.Must(uuid.NewV7())
	store := &fakeOutbox{records: []outbox.Record{{ID: id, Topic: "t", Key: "agg"}}}
	pub := &fakePublisher{}
	relay := NewRelay(store, pub, nil, RelayConfig{WorkerID: "w", BatchSize: 10})
	if err := relay.runBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(pub.published) != 1 || pub.published[0].ID != id {
		t.Fatal("expected one published record")
	}
	if len(store.published) != 1 || store.published[0] != id {
		t.Fatal("expected mark published")
	}
}

func TestRelayMarksFailedOnPublishError(t *testing.T) {
	t.Parallel()

	id := uuid.Must(uuid.NewV7())
	store := &fakeOutbox{records: []outbox.Record{{ID: id, Topic: "t", Key: "agg", Attempts: 1}}}
	relay := NewRelay(store, &fakePublisher{err: errors.New("broker down")}, nil, RelayConfig{WorkerID: "w"})
	if err := relay.runBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.failed) != 1 || store.failed[0] != id {
		t.Fatal("expected mark failed")
	}
	if len(store.published) != 0 {
		t.Fatal("failed publish must not mark published")
	}
}

func TestRelayCrashBetweenPublishAndMarkLeavesEventUnpublished(t *testing.T) {
	t.Parallel()

	id := uuid.Must(uuid.NewV7())
	store := &fakeOutbox{
		records:        []outbox.Record{{ID: id, Topic: "t", Key: "agg"}},
		markPublishErr: errors.New("db unavailable"),
	}
	pub := &fakePublisher{}
	relay := NewRelay(store, pub, nil, RelayConfig{WorkerID: "w"})
	if err := relay.runBatch(context.Background()); err == nil {
		t.Fatal("expected mark published failure")
	}
	if len(pub.published) != 1 {
		t.Fatal("kafka accepted the record")
	}
	if len(store.published) != 0 {
		t.Fatal("outbox row must remain unpublished after crash")
	}
}

func TestRelayRunStopsOnCancel(t *testing.T) {
	t.Parallel()

	relay := NewRelay(&fakeOutbox{}, &fakePublisher{}, nil, RelayConfig{
		WorkerID:     "w",
		PollInterval: time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- relay.Run(ctx) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("relay did not shut down")
	}
}

func TestReplayPreservesEventAndStripsRetryHeaders(t *testing.T) {
	t.Parallel()

	headers := ReplayHeaders([]kgo.RecordHeader{
		{Key: "event-id", Value: []byte("keep")},
		{Key: headerAttempt, Value: []byte("3")},
		{Key: headerNotBefore, Value: []byte("later")},
		{Key: headerErrorClass, Value: []byte("transient")},
		{Key: headerLastFailedAt, Value: []byte("now")},
		{Key: headerSourceTopic, Value: []byte("dev.content.post.events.v1")},
	})
	got := map[string]bool{}
	for _, h := range headers {
		got[h.Key] = true
	}
	if !got["event-id"] || !got[headerSourceTopic] {
		t.Fatal("replay must keep event id and source topic")
	}
	if got[headerAttempt] || got[headerNotBefore] || got[headerErrorClass] || got[headerLastFailedAt] {
		t.Fatal("replay must strip retry failure headers")
	}
}

func TestWaitUntilReadyCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	record := &kgo.Record{
		Headers: []kgo.RecordHeader{{
			Key:   headerNotBefore,
			Value: []byte(time.Now().Add(time.Hour).Format(time.RFC3339Nano)),
		}},
	}
	if err := (&Consumer{}).waitUntilReady(ctx, record); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestRetryDelayBaseGrowsThenCaps(t *testing.T) {
	t.Parallel()
	if retryDelayBase(1) != time.Second {
		t.Fatalf("attempt 1 = %s", retryDelayBase(1))
	}
	if retryDelayBase(2) != 2*time.Second {
		t.Fatalf("attempt 2 = %s", retryDelayBase(2))
	}
	if retryDelayBase(10) != time.Minute {
		t.Fatalf("capped delay = %s", retryDelayBase(10))
	}
}

func testConsumer(t *testing.T, sink RecordPublisher, proc func(context.Context, inbox.Message, event.Event, inbox.Handler) error) *Consumer {
	t.Helper()
	return &Consumer{
		inbox:     inboxFunc(proc),
		publisher: sink,
		handlers: map[string]inbox.Handler{
			event.TypePostCreated: func(context.Context, pgx.Tx, event.Event) error { return nil },
		},
		cfg: ConsumerConfig{
			Name:        "search",
			MaxAttempts: 4,
			RetryDelays: []time.Duration{time.Minute, 10 * time.Minute, time.Hour},
		},
	}
}

func validEvent(t *testing.T) []byte {
	t.Helper()
	e, err := event.New(event.NewParams{
		Source:           "/services/api",
		Type:             event.TypePostCreated,
		Subject:          "urn:post:" + uuid.MustParse("20000000-0000-4000-8000-000000000001").String(),
		DataSchema:       event.SchemaPostCreated,
		AggregateVersion: 1,
		Data:             map[string]any{"post_id": "20000000-0000-4000-8000-000000000001"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

type inboxFunc func(context.Context, inbox.Message, event.Event, inbox.Handler) error

func (f inboxFunc) Process(ctx context.Context, message inbox.Message, e event.Event, handler inbox.Handler) error {
	return f(ctx, message, e, handler)
}

type capturePublisher struct {
	topic  string
	record *kgo.Record
}

func (c *capturePublisher) Publish(_ context.Context, record *kgo.Record) error {
	c.topic = record.Topic
	c.record = record
	return nil
}

type fakePublisher struct {
	err       error
	published []outbox.Record
}

func (f *fakePublisher) Publish(_ context.Context, record outbox.Record) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, record)
	return nil
}

type fakeOutbox struct {
	mu             sync.Mutex
	records        []outbox.Record
	published      []uuid.UUID
	failed         []uuid.UUID
	markPublishErr error
}

func (f *fakeOutbox) Claim(_ context.Context, _ string, _ int, _ time.Duration) ([]outbox.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]outbox.Record(nil), f.records...)
	f.records = nil
	return out, nil
}

func (f *fakeOutbox) MarkPublished(_ context.Context, id uuid.UUID) error {
	if f.markPublishErr != nil {
		return f.markPublishErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published = append(f.published, id)
	return nil
}

func (f *fakeOutbox) MarkFailed(_ context.Context, id uuid.UUID, _ error, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = append(f.failed, id)
	return nil
}

func (f *fakeOutbox) Backlog(context.Context) (int64, time.Duration, error) {
	return int64(len(f.records)), 0, nil
}

func testRecord(t *testing.T, topic string, value []byte) *kgo.Record {
	t.Helper()
	return &kgo.Record{
		Topic: topic,
		Key:   []byte("20000000-0000-4000-8000-000000000001"),
		Value: value,
	}
}
