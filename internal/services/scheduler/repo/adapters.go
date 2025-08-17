package repo

import (
	"context"
	"github.com/NordCoder/Pingerus/internal/domain/check"

	"github.com/NordCoder/Pingerus/internal/domain/kafka"
)

type CheckRepo struct{ R check.Repo }
type Events struct{ P kafka.CheckEvents }

func (a CheckRepo) FetchDue(ctx context.Context, limit int) ([]*check.Check, error) {
	list, err := a.R.FetchDue(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*check.Check, 0, len(list))
	for _, c := range list {
		out = append(out, &check.Check{ID: c.ID})
	}
	return out, nil
}

func (e Events) PublishCheckRequested(ctx context.Context, checkID int64) error {
	return e.P.PublishCheckRequested(ctx, checkID)
}
