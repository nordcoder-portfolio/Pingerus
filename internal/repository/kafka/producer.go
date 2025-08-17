package kafka

import (
	"context"
	"strconv"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

type Producer struct {
	w     *kafka.Writer
	topic string
}

func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		w: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.Hash{},
		},
		topic: topic,
	}
}

func (p *Producer) PublishProto(ctx context.Context, key []byte, m proto.Message) error {
	value, err := proto.Marshal(m)
	if err != nil {
		return err
	}
	return p.w.WriteMessages(ctx, kafka.Message{Key: key, Value: value})
}

func (p *Producer) Close() error { return p.w.Close() }

func KeyFromInt64(id int64) []byte { return []byte(strconv.FormatInt(id, 10)) }
