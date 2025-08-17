package check

import (
	"context"
	"errors"
	"time"

	"github.com/NordCoder/Pingerus/internal/domain/check"
)

var (
	ErrInvalidInterval = errors.New("interval must be >= 10s")
	ErrForbidden       = errors.New("forbidden")
)

type Usecase struct {
	repo check.Repo
}

func NewUsecase(repo check.Repo) *Usecase {
	return &Usecase{repo: repo}
}

func (u *Usecase) Create(ctx context.Context, ownerID int64, url string, interval time.Duration) (*check.Check, error) {
	now := time.Now().UTC()
	c := &check.Check{
		UserID:    ownerID,
		URL:       url,
		Interval:  interval,
		NextRun:   now,
		Active:    true,
		UpdatedAt: now,
	}
	if err := u.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (u *Usecase) Get(ctx context.Context, requesterID int64, id int64) (*check.Check, error) {
	c, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.UserID != requesterID {
		return nil, ErrForbidden
	}
	return c, nil
}

func (u *Usecase) Update(ctx context.Context, requesterID int64, upd *check.Check) (*check.Check, error) {
	cur, err := u.repo.GetByID(ctx, upd.ID)
	if err != nil {
		return nil, err
	}
	if cur.UserID != requesterID {
		return nil, ErrForbidden
	}
	if upd.Interval < 10*time.Second {
		return nil, ErrInvalidInterval
	}
	upd.UserID = requesterID
	upd.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, upd); err != nil {
		return nil, err
	}
	return upd, nil
}

func (u *Usecase) Delete(ctx context.Context, requesterID int64, id int64) error {
	cur, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if cur.UserID != requesterID {
		return ErrForbidden
	}
	return u.repo.Delete(ctx, id)
}

func (u *Usecase) ListByUser(ctx context.Context, requesterID int64) ([]*check.Check, error) {
	return u.repo.ListByUser(ctx, requesterID)
}
