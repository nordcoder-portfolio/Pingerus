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
	log := c.Log.With(zap.String("component", "email-notifier.controller"))
	log.Info("controller run: subscribing to kafka")

	handler := kafkax.ProtoHandler(
		func() *pb.StatusChange { return &pb.StatusChange{} },
		func(ctx context.Context, _ []byte, ev *pb.StatusChange) error {
			checkID := int64(ev.GetCheckId())
			if checkID <= 0 {
				log.Warn("status-change: invalid check_id", zap.Int64("check_id", checkID))
				return nil
			}

			dto := StatusChange{
				CheckID:   checkID,
				OldStatus: ev.GetOldStatus(),
				NewStatus: ev.GetNewStatus(),
				At:        ev.GetTs().AsTime(),
			}

			log.Info("status-change received",
				zap.Int64("check_id", dto.CheckID),
				zap.Bool("old", dto.OldStatus),
				zap.Bool("new", dto.NewStatus),
				zap.Time("at", dto.At),
			)

			if err := c.UC.HandleStatusChange(ctx, dto); err != nil {
				log.Error("handle status-change failed",
					zap.Int64("check_id", dto.CheckID),
					zap.Error(err),
				)
				return err
			}

			log.Info("status-change handled",
				zap.Int64("check_id", dto.CheckID),
			)
			return nil
		},
	)
	return c.Sub.Consume(ctx, handler)
}
