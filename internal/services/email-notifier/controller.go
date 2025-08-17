package notifier

import (
	"context"
	pb "github.com/NordCoder/Pingerus/generated/v1"
	kafkax "github.com/NordCoder/Pingerus/internal/repository/kafka"
	"go.uber.org/zap"
)

type Controller struct {
	Log *zap.Logger
	Sub *kafkax.Consumer
	UC  *Handler
}

func (c *Controller) Run(ctx context.Context) error {
	handler := kafkax.ProtoHandler(
		func() *pb.StatusChange { return &pb.StatusChange{} },
		func(ctx context.Context, _ []byte, ev *pb.StatusChange) error {
			checkID := int64(ev.GetCheckId())
			if checkID <= 0 {
				c.Log.Warn("status-change: invalid check_id", zap.Int64("check_id", checkID))
				return nil
			}
			dto := StatusChange{
				CheckID:   checkID,
				OldStatus: ev.GetOldStatus(),
				NewStatus: ev.GetNewStatus(),
				At:        ev.GetTs().AsTime(),
			}
			return c.UC.HandleStatusChange(ctx, dto)
		},
	)
	return c.Sub.Consume(ctx, handler)
}
