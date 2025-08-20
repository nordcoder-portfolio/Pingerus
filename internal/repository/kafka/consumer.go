package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Handler func(ctx context.Context, key, value []byte) error

type Consumer struct {
	r     *kafka.Reader
	log   *zap.Logger
	topic string
	group string
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
		log:   zap.L().With(zap.String("component", "kafka.consumer")),
		topic: topic,
		group: groupID,
	}
}

func (c *Consumer) WithLogger(l *zap.Logger) *Consumer {
	if l == nil {
		return c
	}
	cp := *c
	cp.log = l.With(
		zap.String("component", "kafka.consumer"),
		zap.String("topic", c.topic),
		zap.String("group", c.group),
	)
	return &cp
}

func (c *Consumer) Consume(ctx context.Context, h Handler) error {
	log := c.log.With(
		zap.String("topic", c.topic),
		zap.String("group", c.group),
	)
	log.Info("consumer started")

	for {
		msg, err := c.r.ReadMessage(ctx)
		if err != nil {
			log.Warn("read message failed", zap.Error(err))
			return err
		}
		log.Debug("message received",
			zap.String("topic", msg.Topic),
			zap.Int("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
			zap.Int("key_len", len(msg.Key)),
			zap.Int("value_len", len(msg.Value)),
		)
		if err := h(ctx, msg.Key, msg.Value); err != nil {
			log.Error("handler error",
				zap.Int("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.Error(err),
			)
			// стратегия на будущее: DLQ/ретраи и т.д.
		}
	}
}

func (c *Consumer) Close() error { return c.r.Close() }
