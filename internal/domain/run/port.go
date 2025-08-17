package run

import "context"

type Repo interface {
	Insert(ctx context.Context, r *Run) error
	ListByCheck(ctx context.Context, checkID int64, limit int) ([]*Run, error)
}
