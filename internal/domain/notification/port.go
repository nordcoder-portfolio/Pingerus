package notification

import "context"

type Repo interface {
	Create(ctx context.Context, n *Notification) error
	ListByUser(ctx context.Context, userID int64, limit int) ([]*Notification, error)
}
