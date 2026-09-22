package eventing

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	"github.com/Yab1/golang-template/internal/platform/event"
	"github.com/Yab1/golang-template/internal/platform/inbox"
	"github.com/Yab1/golang-template/internal/platform/metrics"
)

const (
	headerAttempt       = "x-retry-attempt"
	headerSourceTopic   = "x-source-topic"
	headerNotBefore     = "x-not-before"
	headerConsumer      = "x-failed-consumer"
	headerErrorClass    = "x-error-class"
	headerFirstFailedAt = "x-first-failed-at"
	headerLastFailedAt  = "x-last-failed-at"
)

type ConsumerConfig struct {
	Name        string
	MaxAttempts int
	RetryDelays []time.Duration
}

type Inbox interface {
	Process(ctx context.Context, message inbox.Message, e event.Event, handler inbox.Handler) error
}

type RecordPublisher interface {
	Publish(ctx context.Context, record *kgo.Record) error
}

type kafkaRecordPublisher struct {
	client *kgo.Client
}

func (p kafkaRecordPublisher) Publish(ctx context.Context, record *kgo.Record) error {
	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("publish failed event to %s: %w", record.Topic, err)
	}
	return nil
}

type Consumer struct {
	client    *kgo.Client
	inbox     Inbox
	publisher RecordPublisher
	validator event.SchemaValidator
	handlers  map[string]inbox.Handler
	log       *zap.SugaredLogger
	cfg       ConsumerConfig
}

func NewConsumer(
	client *kgo.Client,
	store *inbox.Store,
	validator event.SchemaValidator,
	handlers map[string]inbox.Handler,
	log *zap.SugaredLogger,
	cfg ConsumerConfig,
) *Consumer {
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 4
	}
	if len(cfg.RetryDelays) == 0 {
		cfg.RetryDelays = []time.Duration{time.Minute, 10 * time.Minute, time.Hour}
	}
	c := &Consumer{
		client:    client,
		inbox:     store,
		validator: validator,
		handlers:  handlers,
		log:       log,
		cfg:       cfg,
	}
	if client != nil {
		c.publisher = kafkaRecordPublisher{client: client}
	}
	return c
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			return fmt.Errorf("poll kafka: %v", errs)
		}
		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()
			if err := c.waitUntilReady(ctx, record); err != nil {
				return err
			}
			if err := c.process(ctx, record); err != nil {
				if c.log != nil {
					c.log.Errorw(
						"event processing failed",
						"consumer", c.cfg.Name,
						"topic", record.Topic,
						"partition", record.Partition,
						"offset", record.Offset,
						"error", err,
					)
				}
				return err
			}
			if err := c.client.CommitRecords(ctx, record); err != nil {
				return fmt.Errorf("commit event offset: %w", err)
			}
		}
	}
}

func (c *Consumer) process(ctx context.Context, record *kgo.Record) error {
	started := time.Now()
	topic := sourceTopic(record)
	err := c.handle(ctx, record)
	result := "ok"
	if err != nil {
		result = "error"
	}
	metrics.ObserveConsumerProcess(c.cfg.Name, topic, result, time.Since(started))
	return err
}

func (c *Consumer) handle(ctx context.Context, record *kgo.Record) error {
	e, err := event.Decode(record.Value)
	if err != nil {
		err = event.Permanent(err)
	}
	if err == nil && c.validator != nil {
		err = c.validator.Validate(ctx, e)
	}
	if err == nil {
		handler, ok := c.handlers[e.Type]
		if !ok {
			err = event.Permanent(fmt.Errorf("unsupported event type %q", e.Type))
		} else {
			err = c.inbox.Process(ctx, inbox.Message{
				ConsumerName: c.cfg.Name,
				EventID:      e.ID,
				Topic:        record.Topic,
				Partition:    record.Partition,
				Offset:       record.Offset,
			}, e, handler)
			switch {
			case errors.Is(err, inbox.ErrDuplicate):
				metrics.ConsumerDuplicate(c.cfg.Name, sourceTopic(record))
				return nil
			case errors.Is(err, inbox.ErrStale):
				metrics.ConsumerStale(c.cfg.Name, sourceTopic(record))
				return nil
			}
		}
	}
	if err == nil {
		metrics.ConsumerProcessed(c.cfg.Name, sourceTopic(record))
		return nil
	}

	attempt := retryAttempt(record) + 1
	if event.IsPermanent(err) || attempt >= c.cfg.MaxAttempts {
		if publishErr := c.publishFailure(ctx, record, event.DLQTopic(sourceTopic(record), c.cfg.Name), attempt, err, 0); publishErr != nil {
			return publishErr
		}
		metrics.ConsumerDeadLettered(c.cfg.Name, sourceTopic(record))
		return nil
	}
	delay := c.retryDelay(attempt)
	if publishErr := c.publishFailure(ctx, record, event.RetryTopic(sourceTopic(record), c.cfg.Name), attempt, err, delay); publishErr != nil {
		return publishErr
	}
	metrics.ConsumerRetried(c.cfg.Name, sourceTopic(record))
	return nil
}

func (c *Consumer) publishFailure(
	ctx context.Context,
	record *kgo.Record,
	topic string,
	attempt int,
	cause error,
	delay time.Duration,
) error {
	now := time.Now().UTC()
	headers := cloneHeaders(record.Headers)
	setHeader(&headers, headerAttempt, strconv.Itoa(attempt))
	setHeader(&headers, headerSourceTopic, sourceTopic(record))
	setHeader(&headers, headerConsumer, c.cfg.Name)
	setHeader(&headers, headerLastFailedAt, now.Format(time.RFC3339Nano))
	if headerValue(record, headerFirstFailedAt) == "" {
		setHeader(&headers, headerFirstFailedAt, now.Format(time.RFC3339Nano))
	}
	errorClass := "transient"
	if event.IsPermanent(cause) {
		errorClass = "permanent"
	}
	setHeader(&headers, headerErrorClass, errorClass)
	if delay > 0 {
		setHeader(&headers, headerNotBefore, now.Add(delay).Format(time.RFC3339Nano))
	}
	return c.publisher.Publish(ctx, &kgo.Record{
		Topic:   topic,
		Key:     record.Key,
		Value:   record.Value,
		Headers: headers,
	})
}

func (c *Consumer) retryDelay(attempt int) time.Duration {
	index := attempt - 1
	if index >= len(c.cfg.RetryDelays) {
		index = len(c.cfg.RetryDelays) - 1
	}
	return c.cfg.RetryDelays[index]
}

func (c *Consumer) waitUntilReady(ctx context.Context, record *kgo.Record) error {
	raw := headerValue(record, headerNotBefore)
	if raw == "" {
		return nil
	}
	notBefore, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil || !notBefore.After(time.Now()) {
		return nil
	}
	timer := time.NewTimer(time.Until(notBefore))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryAttempt(record *kgo.Record) int {
	attempt, _ := strconv.Atoi(headerValue(record, headerAttempt))
	return attempt
}

func sourceTopic(record *kgo.Record) string {
	if source := headerValue(record, headerSourceTopic); source != "" {
		return source
	}
	return record.Topic
}

func headerValue(record *kgo.Record, key string) string {
	for i := len(record.Headers) - 1; i >= 0; i-- {
		if record.Headers[i].Key == key {
			return string(record.Headers[i].Value)
		}
	}
	return ""
}

func cloneHeaders(headers []kgo.RecordHeader) []kgo.RecordHeader {
	cloned := make([]kgo.RecordHeader, len(headers))
	copy(cloned, headers)
	return cloned
}

func setHeader(headers *[]kgo.RecordHeader, key, value string) {
	for i := range *headers {
		if (*headers)[i].Key == key {
			(*headers)[i].Value = []byte(value)
			return
		}
	}
	*headers = append(*headers, kgo.RecordHeader{Key: key, Value: []byte(value)})
}
