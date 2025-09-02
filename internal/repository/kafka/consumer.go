package kafka

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Handler func(ctx context.Context, key, value []byte) error

type Consumer struct {
	reader *kafka.Reader
	log    *zap.Logger
	cfg    *ConsumerConfig
}

type ConsumerConfig struct {
	Brokers       []string
	GroupID       string
	Topic         string
	FromBeginning bool
	Logger        *zap.Logger
}

func NewConsumer(cfg *ConsumerConfig) *Consumer {
	if cfg.Logger == nil {
		cfg.Logger = zap.L()
	}

	start := kafka.LastOffset
	if cfg.FromBeginning {
		start = kafka.FirstOffset
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:               cfg.Brokers,
		GroupID:               cfg.GroupID,
		Topic:                 cfg.Topic,
		StartOffset:           start,
		WatchPartitionChanges: true,

		MinBytes:          1e3,
		MaxBytes:          10e6,
		SessionTimeout:    10 * time.Second,
		RebalanceTimeout:  15 * time.Second,
		HeartbeatInterval: 3 * time.Second,
	})

	log := cfg.Logger.With(
		zap.String("component", "kafka.consumer"),
		zap.String("topic", cfg.Topic),
		zap.String("group", cfg.GroupID),
	)

	return &Consumer{reader: r, log: log, cfg: cfg}
}

func (c *Consumer) WithLogger(l *zap.Logger) *Consumer {
	if l == nil {
		return c
	}
	cp := *c
	cp.log = l.With(
		zap.String("component", "kafka.consumer"),
		zap.String("topic", c.cfg.Topic),
		zap.String("group", c.cfg.GroupID),
	)
	return &cp
}

func (c *Consumer) Consume(ctx context.Context, h Handler) error {
	log := c.log
	log.Info("consumer started")

	backoff := 200 * time.Millisecond
	const maxBackoff = 5 * time.Second

	for {
		select {
		case <-ctx.Done():
			log.Info("consumer stopped (ctx canceled)")
			return ctx.Err()
		default:
		}

		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Info("consumer stopped (ctx canceled)")
				return ctx.Err()
			}
			if errors.Is(err, io.EOF) {
				log.Debug("fetch EOF; retry", zap.Duration("backoff", backoff))
			} else {
				log.Warn("fetch failed; retry", zap.Error(err), zap.Duration("backoff", backoff))
			}
			time.Sleep(backoff)
			if backoff < maxBackoff {
				backoff *= 2
			}
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		backoff = 200 * time.Millisecond

		if err := h(ctx, msg.Key, msg.Value); err != nil {
			log.Error("handler error", zap.Int("partition", msg.Partition), zap.Int64("offset", msg.Offset), zap.Error(err))
			continue
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				log.Info("commit interrupted by context cancel")
				return ctx.Err()
			}
			log.Warn("commit failed; will retry later", zap.Error(err))
		}
	}
}

func (c *Consumer) Close() error { return c.reader.Close() }
