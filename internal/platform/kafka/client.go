package kafka

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
)

type Config struct {
	Brokers  []string
	ClientID string
	Username string
	Password string
	TLS      bool
}

func NewClient(cfg Config, opts ...kgo.Opt) (*kgo.Client, error) {
	base := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordRetries(10),
		kgo.RetryBackoffFn(func(attempt int) time.Duration {
			delay := time.Duration(attempt+1) * 250 * time.Millisecond
			if delay > 5*time.Second {
				return 5 * time.Second
			}
			return delay
		}),
	}
	if cfg.TLS {
		base = append(base, kgo.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	}
	if cfg.Username != "" {
		base = append(base, kgo.SASL(plain.Auth{
			User: cfg.Username,
			Pass: cfg.Password,
		}.AsMechanism()))
	}
	base = append(base, opts...)
	client, err := kgo.NewClient(base...)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}
	return client, nil
}

func Ping(ctx context.Context, client *kgo.Client) error {
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("ping kafka: %w", err)
	}
	return nil
}
