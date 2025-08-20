package kafka

import (
	"context"
	"strconv"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type Producer struct {
	w     *kafka.Writer
	topic string
	log   *zap.Logger
}

func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		w: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Topic:                  topic,
			Balancer:               &kafka.Hash{},
			AllowAutoTopicCreation: true,
		},
		topic: topic,
		log:   zap.L().With(zap.String("component", "kafka.producer"), zap.String("topic", topic)),
	}
}

func (p *Producer) WithLogger(l *zap.Logger) *Producer {
	if l == nil {
		return p
	}
	cp := *p
	cp.log = l.With(zap.String("component", "kafka.producer"), zap.String("topic", p.topic))
	return &cp
}

func (p *Producer) PublishProto(ctx context.Context, key []byte, m proto.Message) error {
	value, err := proto.Marshal(m)
	if err != nil {
		p.log.Error("proto marshal failed", zap.Error(err))
		return err
	}
	err = p.w.WriteMessages(ctx, kafka.Message{Key: key, Value: value})
	if err != nil {
		p.log.Error("kafka write failed", zap.Error(err))
		return err
	}
	p.log.Debug("message published",
		zap.Int("key_len", len(key)),
		zap.Int("value_len", len(value)),
	)
	return nil
}

func (p *Producer) Close() error { return p.w.Close() }

func KeyFromInt64(id int64) []byte { return []byte(strconv.FormatInt(id, 10)) }
