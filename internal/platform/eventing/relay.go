package eventing

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Yab1/golang-template/internal/platform/metrics"
	"github.com/Yab1/golang-template/internal/platform/outbox"
)

type Publisher interface {
	Publish(context.Context, outbox.Record) error
}

type Outbox interface {
	Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]outbox.Record, error)
	MarkPublished(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, cause error, delay time.Duration) error
	Backlog(ctx context.Context) (count int64, oldest time.Duration, err error)
}

type Relay struct {
	outbox       Outbox
	publisher    Publisher
	log          *zap.SugaredLogger
	workerID     string
	batchSize    int
	pollInterval time.Duration
	lease        time.Duration
}

type RelayConfig struct {
	WorkerID     string
	BatchSize    int
	PollInterval time.Duration
	Lease        time.Duration
}

func NewRelay(store Outbox, publisher Publisher, log *zap.SugaredLogger, cfg RelayConfig) *Relay {
	if cfg.BatchSize < 1 {
		cfg.BatchSize = 100
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.Lease <= 0 {
		cfg.Lease = 30 * time.Second
	}
	return &Relay{
		outbox:       store,
		publisher:    publisher,
		log:          log,
		workerID:     cfg.WorkerID,
		batchSize:    cfg.BatchSize,
		pollInterval: cfg.PollInterval,
		lease:        cfg.Lease,
	}
}

func (r *Relay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()
	for {
		if err := r.runBatch(ctx); err != nil && r.log != nil {
			r.log.Errorw("outbox relay batch failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (r *Relay) runBatch(ctx context.Context) error {
	records, err := r.outbox.Claim(ctx, r.workerID, r.batchSize, r.lease)
	if err != nil {
		return fmt.Errorf("claim outbox: %w", err)
	}
	for _, record := range records {
		started := time.Now()
		if err := r.publisher.Publish(ctx, record); err != nil {
			metrics.ObserveOutboxPublish(record.Topic, time.Since(started), false)
			if markErr := r.outbox.MarkFailed(ctx, record.ID, err, retryDelay(record.Attempts)); markErr != nil {
				return fmt.Errorf("mark event %s failed after publish error: %w", record.ID, markErr)
			}
			continue
		}
		if err := r.outbox.MarkPublished(ctx, record.ID); err != nil {
			metrics.ObserveOutboxPublish(record.Topic, time.Since(started), false)
			return fmt.Errorf("mark event %s published: %w", record.ID, err)
		}
		metrics.ObserveOutboxPublish(record.Topic, time.Since(started), true)
	}
	count, oldest, err := r.outbox.Backlog(ctx)
	if err == nil {
		metrics.SetOutboxBacklog(count, oldest.Seconds())
	}
	return nil
}

func retryDelay(attempt int) time.Duration {
	base := retryDelayBase(attempt)
	if base <= 0 {
		return base
	}
	jitter := time.Duration(rand.Int64N(int64(base/4) + 1))
	return base + jitter
}

func retryDelayBase(attempt int) time.Duration {
	seconds := math.Pow(2, float64(max(attempt-1, 0)))
	delay := time.Duration(seconds * float64(time.Second))
	if delay > time.Minute {
		return time.Minute
	}
	return delay
}
