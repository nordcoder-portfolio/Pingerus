package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type Handler func(ctx context.Context, key, value []byte) error

type Consumer struct {
	r *kafka.Reader
}

func NewConsumer(brokers []string, groupID, topic string) *Consumer {
	return &Consumer{
		r: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			GroupID:  groupID,
			Topic:    topic,
			MinBytes: 1e3,
			MaxBytes: 10e6,
		}),
	}
}

func (c *Consumer) Consume(ctx context.Context, h Handler) error {
	for {
		msg, err := c.r.ReadMessage(ctx)
		if err != nil {
			return err
		}
		if err := h(ctx, msg.Key, msg.Value); err != nil {
			// todo think about advanced strats
		}
	}
}

func (c *Consumer) Close() error { return c.r.Close() }
