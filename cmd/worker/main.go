package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	"github.com/Yab1/golang-template/internal/platform/blob"
	"github.com/Yab1/golang-template/internal/platform/config"
	"github.com/Yab1/golang-template/internal/platform/db"
	"github.com/Yab1/golang-template/internal/platform/event"
	"github.com/Yab1/golang-template/internal/platform/eventing"
	"github.com/Yab1/golang-template/internal/platform/inbox"
	kafkaplatform "github.com/Yab1/golang-template/internal/platform/kafka"
	"github.com/Yab1/golang-template/internal/platform/logger"
	"github.com/Yab1/golang-template/internal/platform/metrics"
	"github.com/Yab1/golang-template/internal/platform/outbox"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

func main() {
	cfg := config.Load()
	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()
	if !cfg.Kafka.Enabled {
		log.Fatal("worker requires KAFKA_ENABLED=true")
	}

	pool, err := db.New(cfg.DB.Addr, cfg.DB.MaxConns, cfg.DB.MinConns, cfg.DB.MaxConnIdleTime)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := storage.SchemaCurrent(ctx, pool, cfg.DB.ExpectedVersion); err != nil {
		log.Fatalw("database schema is not ready", "error", err)
	}

	kafkaCfg := kafkaplatform.Config{
		Brokers:  cfg.Kafka.Brokers,
		ClientID: cfg.Kafka.ClientID + "-worker",
		Username: cfg.Kafka.Username,
		Password: cfg.Kafka.Password,
		TLS:      cfg.Kafka.TLS,
	}
	producerClient, err := kafkaplatform.NewClient(kafkaCfg)
	if err != nil {
		log.Fatal(err)
	}
	defer producerClient.Close()
	if err := kafkaplatform.Ping(ctx, producerClient); err != nil {
		log.Fatal(err)
	}
	validator := event.NewRegistryValidator(cfg.Kafka.SchemaRegistryURL, nil)
	if err := validator.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	errCh := make(chan error, 3)
	startHealthServer(ctx, cfg, pool, producerClient, validator, log, errCh)

	blobStore, err := blob.New(cfg.Storage)
	if err != nil {
		log.Fatal(err)
	}

	mode := strings.ToLower(cfg.Eventing.WorkerMode)
	if mode == "all" || mode == "relay" {
		relay := eventing.NewRelay(
			outbox.NewStore(pool),
			kafkaplatform.NewProducer(producerClient),
			log,
			eventing.RelayConfig{
				WorkerID:     hostname(),
				BatchSize:    cfg.Eventing.BatchSize,
				PollInterval: cfg.Eventing.PollInterval,
				Lease:        cfg.Eventing.ClaimLease,
			},
		)
		go func() { errCh <- relay.Run(ctx) }()
	}

	var consumerClient *kgo.Client
	if mode == "all" || mode == "consumer" {
		topics := sourceTopics(cfg.Kafka.TopicPrefix)
		consumeTopics := append([]string{}, topics...)
		for _, topic := range topics {
			consumeTopics = append(consumeTopics, event.RetryTopic(topic, cfg.Eventing.ConsumerName))
		}
		consumerClient, err = kafkaplatform.NewClient(
			kafkaCfg,
			kgo.ConsumerGroup(cfg.Kafka.TopicPrefix+"."+cfg.Eventing.ConsumerName),
			kgo.ConsumeTopics(consumeTopics...),
			kgo.DisableAutoCommit(),
		)
		if err != nil {
			log.Fatal(err)
		}
		defer consumerClient.Close()
		consumer := eventing.NewConsumer(
			consumerClient,
			inbox.NewStore(pool),
			validator,
			logHandlers(log, blobStore),
			log,
			eventing.ConsumerConfig{
				Name:        cfg.Eventing.ConsumerName,
				MaxAttempts: cfg.Eventing.MaxAttempts,
				RetryDelays: cfg.Eventing.RetryDelays,
			},
		)
		go func() { errCh <- consumer.Run(ctx) }()
	}

	if mode != "all" && mode != "relay" && mode != "consumer" {
		log.Fatalf("unsupported EVENT_WORKER_MODE %q", cfg.Eventing.WorkerMode)
	}

	select {
	case <-ctx.Done():
	case runErr := <-errCh:
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
			log.Errorw("worker stopped", "error", runErr)
		}
		cancel()
	}

}

func sourceTopics(prefix string) []string {
	return []string{
		event.Topic(prefix, "identity", "user"),
		event.Topic(prefix, "content", "post"),
		event.Topic(prefix, "files", "file"),
	}
}

func logHandlers(log *zap.SugaredLogger, blobStore blob.Store) map[string]inbox.Handler {
	types := []string{
		event.TypeUserRegistered,
		event.TypeUserActivated,
		event.TypeUserPasswordChanged,
		event.TypeUserVisibilityChanged,
		event.TypePostCreated,
		event.TypePostUpdated,
		event.TypePostVisibilityChanged,
		event.TypePostDeleted,
		event.TypeFileUploaded,
		event.TypeFileDeleted,
	}
	handlers := make(map[string]inbox.Handler, len(types))
	for _, eventType := range types {
		handlers[eventType] = func(_ context.Context, _ pgx.Tx, e event.Event) error {
			log.Infow("event processed", "event_id", e.ID, "event_type", e.Type, "subject", e.Subject)
			return nil
		}
	}
	handlers[event.TypeFileDeleted] = func(ctx context.Context, _ pgx.Tx, e event.Event) error {
		var data struct {
			FileID string `json:"file_id"`
		}
		if err := json.Unmarshal(e.Data, &data); err != nil {
			return event.Permanent(fmt.Errorf("decode file deleted data: %w", err))
		}
		if data.FileID == "" {
			return event.Permanent(errors.New("file delete key is required"))
		}
		if err := blobStore.Delete(ctx, data.FileID); err != nil && !os.IsNotExist(err) {
			return err
		}
		log.Infow("file object deleted", "event_id", e.ID, "key", data.FileID)
		return nil
	}
	return handlers
}

func startHealthServer(
	ctx context.Context,
	cfg config.Config,
	pool interface {
		Ping(context.Context) error
	},
	client *kgo.Client,
	validator *event.RegistryValidator,
	log *zap.SugaredLogger,
	errCh chan<- error,
) {
	mux := http.NewServeMux()
	mux.HandleFunc("/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := kafkaplatform.Ping(r.Context(), client); err != nil {
			http.Error(w, "kafka unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := validator.Ping(r.Context()); err != nil {
			http.Error(w, "schema registry unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	if cfg.MetricsExposed() {
		mux.Handle("/metrics", metrics.Protect(cfg.Metrics.Token)(metrics.Handler()))
	}
	server := &http.Server{
		Addr:              cfg.Eventing.WorkerAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Eventing.ShutdownTimeout)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	go func() {
		log.Infow("event worker health server started", "addr", cfg.Eventing.WorkerAddr)
		err := server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("worker health server: %w", err)
		}
	}()
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "event-worker"
	}
	return name
}
