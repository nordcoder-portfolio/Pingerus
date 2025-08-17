package kafka

import (
	"context"

	pb "github.com/NordCoder/Pingerus/generated/v1"
	"github.com/NordCoder/Pingerus/internal/domain/kafka"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CheckEventsKafka struct {
	p *Producer
}

func NewCheckEventsKafka(p *Producer) *CheckEventsKafka { return &CheckEventsKafka{p: p} }

var _ kafka.CheckEvents = (*CheckEventsKafka)(nil)

func (e *CheckEventsKafka) PublishCheckRequested(ctx context.Context, checkID int64) error {
	return e.p.PublishProto(ctx, KeyFromInt64(checkID), &pb.CheckRequest{
		CheckId: int32(checkID),
	})
}

func (e *CheckEventsKafka) PublishStatusChanged(ctx context.Context, checkID int64, old, new bool) error {
	return e.p.PublishProto(ctx, KeyFromInt64(checkID), &pb.StatusChange{
		CheckId:   int32(checkID),
		OldStatus: old,
		NewStatus: new,
		Ts:        timestamppb.Now(),
	})
}
