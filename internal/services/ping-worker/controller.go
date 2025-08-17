package ping_worker

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
		func() *pb.CheckRequest { return &pb.CheckRequest{} },
		func(ctx context.Context, _ []byte, msg *pb.CheckRequest) error {
			c.Log.Debug("check-request", zap.Int64("check_id", int64(msg.GetCheckId())))
			cid := int64(msg.GetCheckId())
			return c.UC.HandleCheck(ctx, cid)
		},
	)
	return c.Sub.Consume(ctx, handler)
}
