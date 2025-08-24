package kafka

import (
	"context"
	"errors"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"io"
	"time"

	"go.uber.org/zap"
)

type Handler func(ctx context.Context, key, value []byte) error

type Consumer struct {
	r     *kafka.Reader
	log   *zap.Logger
	topic string
	group string
}

type ConsumerConfig struct {
	Brokers           []string
	GroupID           string
	Topic             string
	StartFromEarliest bool
	MinBytes          int
	MaxBytes          int
	SessionTimeout    time.Duration
	RebalanceTimeout  time.Duration
	HeartbeatInterval time.Duration
	CommitInterval    time.Duration
	Logger            *zap.Logger
}

func NewConsumerWithConfig(cfg ConsumerConfig) *Consumer {
	if cfg.MinBytes <= 0 {
		cfg.MinBytes = 1e3
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 10e6
	}

	start := kafka.LastOffset
	if cfg.StartFromEarliest {
		start = kafka.FirstOffset
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:               cfg.Brokers,
		GroupID:               cfg.GroupID,
		Topic:                 cfg.Topic,
		MinBytes:              cfg.MinBytes,
		MaxBytes:              cfg.MaxBytes,
		StartOffset:           start,
		WatchPartitionChanges: true,
		SessionTimeout:        orDur(cfg.SessionTimeout, 10*time.Second),
		RebalanceTimeout:      orDur(cfg.RebalanceTimeout, 15*time.Second),
		HeartbeatInterval:     orDur(cfg.HeartbeatInterval, 3*time.Second),
		CommitInterval:        cfg.CommitInterval,
	})

	log := cfg.Logger
	if log == nil {
		log = zap.L()
	}
	log = log.With(zap.String("component", "kafka.consumer"),
		zap.String("topic", cfg.Topic), zap.String("group", cfg.GroupID))

	return &Consumer{r: r, log: log, topic: cfg.Topic, group: cfg.GroupID}
}

func NewConsumer(brokers []string, groupID, topic string) *Consumer {
	return NewConsumerWithConfig(ConsumerConfig{
		Brokers:           brokers,
		GroupID:           groupID,
		Topic:             topic,
		StartFromEarliest: true,
	})
}

func (c *Consumer) WithLogger(l *zap.Logger) *Consumer {
	if l == nil {
		return c
	}
	cp := *c
	cp.log = l.With(zap.String("component", "kafka.consumer"),
		zap.String("topic", c.topic), zap.String("group", c.group))
	return &cp
}

func (c *Consumer) Consume(ctx context.Context, h Handler) error {
	log := c.log
	log.Info("consumer started")

	backoff := 200 * time.Millisecond
	const maxBackoff = 10 * time.Second

	for {
		select {
		case <-ctx.Done():
			log.Info("consumer stopped (ctx canceled)")
			return ctx.Err()
		default:
		}

		msg, err := c.r.FetchMessage(ctx)
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

		prop := otel.GetTextMapPropagator()
		parent := prop.Extract(context.Background(), mapCarrierFromKafka(msg.Headers))

		tr := otel.Tracer("kafka.consumer")
		ctxCons, cons := tr.Start(parent, "kafka.consume "+msg.Topic, trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				semconv.MessagingSystemKafka,
				semconv.MessagingDestinationName(msg.Topic),
				semconv.MessagingOperationReceive,
				attribute.Int("kafka.partition", msg.Partition),
				attribute.Int64("kafka.offset", msg.Offset),
			),
		)
		cons.End()

		ctxProc, span := tr.Start(ctxCons, "process "+msg.Topic)

		if err := h(ctxProc, msg.Key, msg.Value); err != nil {
			log.Error("handler error", zap.Int("partition", msg.Partition),
				zap.Int64("offset", msg.Offset), zap.Error(err))
			span.End()
			continue
		}

		if err := c.r.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				log.Info("commit interrupted by context cancel")
				return ctx.Err()
			}
			log.Warn("commit failed; will retry later", zap.Error(err))
			time.Sleep(time.Second)
			span.End()
			continue
		}
		span.End()
	}
}

func (c *Consumer) Close() error { return c.r.Close() }

func orDur(v, def time.Duration) time.Duration {
	if v <= 0 {
		return def
	}
	return v
}
