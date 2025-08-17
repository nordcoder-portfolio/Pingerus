package repo

import (
	"context"
	"github.com/NordCoder/Pingerus/internal/domain/check"
	"github.com/NordCoder/Pingerus/internal/domain/run"
	"time"

	"github.com/NordCoder/Pingerus/internal/repository/kafka"
)

type CheckRepo struct{ R check.Repo }
type RunRepo struct{ R run.Repo }
type Events struct{ P *kafka.CheckEventsKafka }

func (a CheckRepo) GetByID(ctx context.Context, id int64) (*check.Check, error) {
	c, err := a.R.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &check.Check{
		ID:         c.ID,
		UserID:     c.UserID,
		URL:        c.URL,
		LastStatus: c.LastStatus,
	}, nil
}
func (a CheckRepo) Update(ctx context.Context, c *check.Check) error {
	return a.R.Update(ctx, &check.Check{
		ID:         c.ID,
		UserID:     c.UserID,
		URL:        c.URL,
		LastStatus: c.LastStatus,
		UpdatedAt:  time.Now().UTC(), // как и раньше
		Active:     true,
	})
}

func (a RunRepo) Insert(ctx context.Context, r *run.Run) error {
	return a.R.Insert(ctx, &run.Run{
		CheckID:   r.CheckID,
		Timestamp: r.Timestamp,
		Status:    r.Status,
		Code:      r.Code,
		Latency:   r.Latency,
	})
}

func (e Events) PublishStatusChanged(ctx context.Context, checkID int64, oldStatus, newStatus bool) error {
	return e.P.PublishStatusChanged(ctx, checkID, oldStatus, newStatus)
}
