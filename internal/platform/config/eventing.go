package config

import (
	"time"

	"github.com/Yab1/golang-template/internal/platform/env"
)

type Kafka struct {
	Enabled           bool
	Brokers           []string
	ClientID          string
	Username          string
	Password          string
	TLS               bool
	SchemaRegistryURL string
	TopicPrefix       string
}

type Eventing struct {
	WorkerAddr      string
	WorkerMode      string
	ConsumerName    string
	BatchSize       int
	PollInterval    time.Duration
	ClaimLease      time.Duration
	MaxAttempts     int
	RetryDelays     []time.Duration
	ShutdownTimeout time.Duration
}

func loadKafka(envName, appName string) Kafka {
	return Kafka{
		Enabled:           env.GetBool("KAFKA_ENABLED", false),
		Brokers:           env.GetCSV("KAFKA_BROKERS", []string{"localhost:9092"}),
		ClientID:          env.GetString("KAFKA_CLIENT_ID", appName),
		Username:          env.GetString("KAFKA_USERNAME", ""),
		Password:          env.GetString("KAFKA_PASSWORD", ""),
		TLS:               env.GetBool("KAFKA_TLS", envName != "development"),
		SchemaRegistryURL: env.GetString("KAFKA_SCHEMA_REGISTRY_URL", "http://localhost:8081"),
		TopicPrefix:       env.GetString("KAFKA_TOPIC_PREFIX", envName+"."+appName),
	}
}

func loadEventing() Eventing {
	return Eventing{
		WorkerAddr:   env.GetString("EVENT_WORKER_ADDR", ":8081"),
		WorkerMode:   env.GetString("EVENT_WORKER_MODE", "all"),
		ConsumerName: env.GetString("EVENT_CONSUMER_NAME", "event-logger"),
		BatchSize:    env.GetInt("EVENT_OUTBOX_BATCH_SIZE", 100),
		PollInterval: env.GetDuration("EVENT_OUTBOX_POLL_INTERVAL", "1s"),
		ClaimLease:   env.GetDuration("EVENT_OUTBOX_CLAIM_LEASE", "30s"),
		MaxAttempts:  env.GetInt("EVENT_CONSUMER_MAX_ATTEMPTS", 4),
		RetryDelays: []time.Duration{
			env.GetDuration("EVENT_RETRY_DELAY_1", "1m"),
			env.GetDuration("EVENT_RETRY_DELAY_2", "10m"),
			env.GetDuration("EVENT_RETRY_DELAY_3", "1h"),
		},
		ShutdownTimeout: env.GetDuration("EVENT_WORKER_SHUTDOWN_TIMEOUT", "30s"),
	}
}
