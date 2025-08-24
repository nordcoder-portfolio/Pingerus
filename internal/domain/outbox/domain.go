package outbox

import (
	"context"
	"time"
)

type Status string

type Kind int

const (
	KindStatusChanged Kind = 1
)

type Message struct {
	IdempotencyKey string
	Kind           Kind
	Data           []byte
	Status         Status
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Tracestate     string
	Traceparent    string
	Baggage        string
}

type Repository interface {
	Enqueue(ctx context.Context, key string, kind Kind, data []byte) error

	PickBatch(ctx context.Context, batch int, inProgressTTL time.Duration) ([]Message, error)

	MarkSuccess(ctx context.Context, keys []string) error
}

type KindHandler func(ctx context.Context, data []byte) error

type GlobalHandler func(kind Kind) (KindHandler, error)
