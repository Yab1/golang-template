package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/Yab1/golang-template/internal/platform/config"
	"github.com/Yab1/golang-template/internal/platform/eventing"
	kafkaplatform "github.com/Yab1/golang-template/internal/platform/kafka"
	"github.com/Yab1/golang-template/internal/platform/logger"
)

func main() {
	var (
		topic = flag.String("topic", "", "DLQ topic to replay")
		group = flag.String("group", fmt.Sprintf("dlq-replay-%d", time.Now().Unix()), "replay consumer group")
		limit = flag.Int("limit", 0, "maximum events to replay; zero means unlimited")
	)
	flag.Parse()
	if *topic == "" {
		fmt.Fprintln(os.Stderr, "-topic is required")
		os.Exit(2)
	}

	cfg := config.Load()
	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()

	client, err := kafkaplatform.NewClient(
		kafkaplatform.Config{
			Brokers:  cfg.Kafka.Brokers,
			ClientID: cfg.Kafka.ClientID + "-replay",
			Username: cfg.Kafka.Username,
			Password: cfg.Kafka.Password,
			TLS:      cfg.Kafka.TLS,
		},
		kgo.ConsumerGroup(*group),
		kgo.ConsumeTopics(*topic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	replayed := 0
	for ctx.Err() == nil {
		fetches := client.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			log.Fatalw("poll DLQ", "errors", errs)
		}
		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()
			source := header(record, "x-source-topic")
			if source == "" {
				log.Fatalw("DLQ event missing source topic", "topic", record.Topic, "offset", record.Offset)
			}
			result := client.ProduceSync(ctx, &kgo.Record{
				Topic:   source,
				Key:     record.Key,
				Value:   record.Value,
				Headers: eventing.ReplayHeaders(record.Headers),
			})
			if err := result.FirstErr(); err != nil {
				log.Fatalw("replay event", "error", err)
			}
			if err := client.CommitRecords(ctx, record); err != nil {
				log.Fatalw("commit DLQ offset", "error", err)
			}
			replayed++
			log.Infow("DLQ event replayed", "source_topic", source, "offset", record.Offset)
			if *limit > 0 && replayed >= *limit {
				return
			}
		}
	}
}

func header(record *kgo.Record, key string) string {
	for i := len(record.Headers) - 1; i >= 0; i-- {
		if record.Headers[i].Key == key {
			return string(record.Headers[i].Value)
		}
	}
	return ""
}
