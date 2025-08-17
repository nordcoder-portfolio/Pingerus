package kafka

import "context"

type CheckEvents interface {
	PublishCheckRequested(ctx context.Context, checkID int64) error
	PublishStatusChanged(ctx context.Context, checkID int64, old, new bool) error
}
