package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	outboxPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "event_outbox_published_total",
		Help: "Outbox events published by topic",
	}, []string{"topic"})

	outboxPublishFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "event_outbox_publish_failures_total",
		Help: "Outbox publish failures by topic",
	}, []string{"topic"})

	outboxBacklog = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "event_outbox_backlog",
		Help: "Number of unpublished outbox events",
	})

	outboxOldestSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "event_outbox_oldest_seconds",
		Help: "Age of oldest unpublished outbox event",
	})

	consumerProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "event_consumer_processed_total",
		Help: "Events processed by consumer and topic",
	}, []string{"consumer", "topic"})

	consumerRetries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "event_consumer_retries_total",
		Help: "Events sent to retry by consumer and topic",
	}, []string{"consumer", "topic"})

	consumerDLQ = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "event_consumer_dlq_total",
		Help: "Events sent to DLQ by consumer and topic",
	}, []string{"consumer", "topic"})

	consumerDuplicates = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "event_consumer_duplicates_total",
		Help: "Duplicate events ignored by consumer",
	}, []string{"consumer", "topic"})

	consumerStale = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "event_consumer_stale_total",
		Help: "Stale aggregate versions ignored by consumer",
	}, []string{"consumer", "topic"})

	outboxPublishDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "event_outbox_publish_duration_seconds",
		Help:    "Outbox publish duration by topic",
		Buckets: prometheus.DefBuckets,
	}, []string{"topic", "result"})

	consumerProcessDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "event_consumer_process_duration_seconds",
		Help:    "Consumer processing duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"consumer", "topic", "result"})
)

func OutboxPublished(topic string) {
	outboxPublished.WithLabelValues(topic).Inc()
}

func OutboxPublishFailed(topic string) {
	outboxPublishFailures.WithLabelValues(topic).Inc()
}

func SetOutboxBacklog(count int64, oldestSeconds float64) {
	outboxBacklog.Set(float64(count))
	outboxOldestSeconds.Set(oldestSeconds)
}

func ConsumerProcessed(consumer, topic string) {
	consumerProcessed.WithLabelValues(consumer, topic).Inc()
}

func ConsumerRetried(consumer, topic string) {
	consumerRetries.WithLabelValues(consumer, topic).Inc()
}

func ConsumerDeadLettered(consumer, topic string) {
	consumerDLQ.WithLabelValues(consumer, topic).Inc()
}

func ObserveOutboxPublish(topic string, d time.Duration, ok bool) {
	result := "error"
	if ok {
		result = "ok"
		OutboxPublished(topic)
	} else {
		OutboxPublishFailed(topic)
	}
	outboxPublishDuration.WithLabelValues(topic, result).Observe(d.Seconds())
}

func ObserveConsumerProcess(consumer, topic, result string, d time.Duration) {
	consumerProcessDuration.WithLabelValues(consumer, topic, result).Observe(d.Seconds())
}

func ConsumerStale(consumer, topic string) {
	consumerStale.WithLabelValues(consumer, topic).Inc()
}

func ConsumerDuplicate(consumer, topic string) {
	consumerDuplicates.WithLabelValues(consumer, topic).Inc()
}
