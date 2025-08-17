package check

import "context"

type Repo interface {
	Create(ctx context.Context, c *Check) error
	GetByID(ctx context.Context, id int64) (*Check, error)
	ListByUser(ctx context.Context, userID int64) ([]*Check, error)
	Update(ctx context.Context, c *Check) error
	Delete(ctx context.Context, id int64) error
	FetchDue(ctx context.Context, limit int) ([]*Check, error)
}
