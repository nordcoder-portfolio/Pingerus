package repo

import (
	"context"
	"github.com/NordCoder/Pingerus/internal/domain/check"
	"github.com/NordCoder/Pingerus/internal/domain/notification"
	"github.com/NordCoder/Pingerus/internal/domain/user"
)

type CheckReader struct{ R check.Repo }
type UserReader struct{ R user.Repo }
type NotificationRepo struct{ R notification.Repo }

func (a CheckReader) GetByID(ctx context.Context, id int64) (*check.Check, error) {
	c, err := a.R.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &check.Check{ID: c.ID, UserID: c.UserID, URL: c.URL}, nil
}
func (a UserReader) GetByID(ctx context.Context, id int64) (*user.User, error) {
	u, err := a.R.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &user.User{ID: u.ID, Email: u.Email}, nil
}
func (a NotificationRepo) Create(ctx context.Context, n *notification.Notification) error {
	return a.R.Create(ctx, &notification.Notification{
		CheckID: n.CheckID, UserID: n.UserID, Type: n.Type,
		SentAt: n.SentAt, Payload: n.Payload,
	})
}
