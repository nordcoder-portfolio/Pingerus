package notifier

import (
	"context"
	"errors"
	"fmt"
	"time"

	pb "github.com/NordCoder/Pingerus/generated/v1"
	kafkax "github.com/NordCoder/Pingerus/internal/repository/kafka"
	"go.uber.org/zap"
)

type Controller struct {
	Log *zap.Logger
	Sub *kafkax.Consumer
	UC  *Handler
}

func (c *Controller) logger() *zap.Logger {
	if c.Log != nil {
		return c.Log
	}
	return zap.NewNop()
}

func (c *Controller) Run(ctx context.Context) error {
	log := c.logger().With(zap.String("component", "email-notifier.controller"))
	log.Info("subscribing to kafka")

	handler := kafkax.ProtoHandler(
		func() *pb.StatusChange { return &pb.StatusChange{} },
		func(parent context.Context, _ []byte, ev *pb.StatusChange) (err error) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("panic in handler", zap.Any("panic", r))
					if err == nil {
						err = fmt.Errorf("panic: %v", r)
					}
				}
			}()

			checkID := int64(ev.GetCheckId())
			if checkID <= 0 {
				log.Warn("status-change: invalid check_id", zap.Int64("check_id", checkID))
				return nil
			}

			ctxMsg, cancel := context.WithTimeout(parent, 10*time.Second)
			defer cancel()

			ts := ev.GetTs().AsTime()
			if ts.IsZero() {
				ts = time.Now().UTC()
			}
			dto := StatusChange{
				CheckID:   checkID,
				OldStatus: ev.GetOldStatus(),
				NewStatus: ev.GetNewStatus(),
				At:        ts,
			}

			if dto.OldStatus == dto.NewStatus {
				log.Debug("no-op status-change (old==new)", zap.Int64("check_id", dto.CheckID))
				return nil
			}

			clog := log.With(
				zap.Int64("check_id", dto.CheckID),
				zap.Bool("old", dto.OldStatus),
				zap.Bool("new", dto.NewStatus),
				zap.Time("at", dto.At),
			)
			clog.Debug("status-change received")

			if err := c.UC.HandleStatusChange(ctxMsg, dto); err != nil {
				clog.Error("handle status-change failed", zap.Error(err))
				return err
			}
			clog.Debug("status-change handled")
			return nil
		},
	)

	if err := c.Sub.Consume(ctx, handler); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Info("controller stopped (context canceled)")
			return nil
		}
		return err
	}
	return nil
}
