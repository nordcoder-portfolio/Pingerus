package kafka

import (
	"context"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type TopicSpec struct {
	Name              string
	NumPartitions     int
	ReplicationFactor int
	MaxWait           time.Duration
}

func EnsureTopic(ctx context.Context, brokers []string, spec TopicSpec, log *zap.Logger) error {
	if spec.NumPartitions <= 0 {
		spec.NumPartitions = 1
	}
	if spec.ReplicationFactor <= 0 {
		spec.ReplicationFactor = 1
	}
	if spec.MaxWait <= 0 {
		spec.MaxWait = 5 * time.Second
	}

	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		if log != nil {
			log.Warn("kafka dial failed", zap.Error(err))
		}
		return err
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		if log != nil {
			log.Warn("kafka controller", zap.Error(err))
		}
		return err
	}
	cc, err := kafka.DialContext(ctx, "tcp", controller.Host+":"+strconv.Itoa(controller.Port))
	if err != nil {
		if log != nil {
			log.Warn("kafka dial controller", zap.Error(err))
		}
		return err
	}
	defer cc.Close()

	err = cc.CreateTopics(kafka.TopicConfig{
		Topic:             spec.Name,
		NumPartitions:     spec.NumPartitions,
		ReplicationFactor: spec.ReplicationFactor,
	})
	if err != nil {
		if log != nil {
			log.Debug("create topic (maybe exists)", zap.String("topic", spec.Name), zap.Error(err))
		}
	}

	deadline := time.Now().Add(spec.MaxWait)
	for time.Now().Before(deadline) {
		ps, err := conn.ReadPartitions(spec.Name)
		if err == nil && len(ps) > 0 {
			if log != nil {
				log.Info("topic ready", zap.String("topic", spec.Name))
			}
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	if log != nil {
		log.Warn("topic not confirmed ready in time", zap.String("topic", spec.Name))
	}
	return nil
}
