package notification

import (
	"context"
	"time"
)

type Notification struct {
	ID      int64     `json:"id"`
	CheckID int64     `json:"check_id"`
	UserID  int64     `json:"user_id"`
	Type    string    `json:"type"`
	SentAt  time.Time `json:"sent_at"`
	Payload string    `json:"payload"`
}

type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

type Clock interface {
	Now() time.Time
}
