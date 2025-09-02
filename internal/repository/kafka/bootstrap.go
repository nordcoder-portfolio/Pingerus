package kafka

import (
	"context"
	"go.uber.org/zap"
	"time"
)

func BootstrapConsumer(ctx context.Context, cfg *ConsumerConfig, logger *zap.Logger) *Consumer {
	_ = EnsureTopic(ctx, cfg.Brokers, TopicSpec{
		Name:              cfg.Topic, // todo config
		NumPartitions:     1,
		ReplicationFactor: 1,
		MaxWait:           5 * time.Second,
	}, logger)

	return NewConsumer(cfg)
}
