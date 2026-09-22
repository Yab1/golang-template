package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/Yab1/golang-template/internal/platform/outbox"
)

type Producer struct {
	client *kgo.Client
}

func NewProducer(client *kgo.Client) *Producer {
	return &Producer{client: client}
}

func (p *Producer) Publish(ctx context.Context, record outbox.Record) error {
	headers := make([]kgo.RecordHeader, 0, len(record.Headers)+3)
	for key, value := range record.Headers {
		headers = append(headers, kgo.RecordHeader{Key: key, Value: []byte(value)})
	}
	headers = append(headers,
		kgo.RecordHeader{Key: "event-id", Value: []byte(record.ID.String())},
		kgo.RecordHeader{Key: "event-type", Value: []byte(record.EventType)},
		kgo.RecordHeader{Key: "schema-uri", Value: []byte(record.SchemaURI)},
	)
	result := p.client.ProduceSync(ctx, &kgo.Record{
		Topic:   record.Topic,
		Key:     []byte(record.Key),
		Value:   record.Payload,
		Headers: headers,
	})
	if err := result.FirstErr(); err != nil {
		return fmt.Errorf("publish event %s: %w", record.ID, err)
	}
	return nil
}
