package scheduler

import (
	"context"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/services/scheduler/repo"
)

type Usecase struct {
	Repo   repo.CheckRepo
	Events repo.Events
}

func NewUC(repo repo.CheckRepo, events repo.Events) *Usecase {
	return &Usecase{Repo: repo, Events: events}
}

func (u *Usecase) Tick(ctx context.Context, limit int) (int, int, int, error) {
	if limit <= 0 {
		limit = 100
	}
	due, err := u.Repo.FetchDue(ctx, limit)
	if err != nil {
		return 0, 0, 1, fmt.Errorf("fetch due: %w", err)
	}
	if len(due) == 0 {
		return 0, 0, 0, nil
	}

	sent, errs := 0, 0
	for _, c := range due {
		if err := u.Events.PublishCheckRequested(ctx, c.ID); err != nil {
			errs++
			continue
		}
		sent++
	}
	return len(due), sent, errs, nil
}
