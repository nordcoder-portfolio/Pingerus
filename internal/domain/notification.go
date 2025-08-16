package domain

import "time"

type Notification struct {
	ID      int64     `json:"id"`
	CheckID int64     `json:"check_id"`
	UserID  int64     `json:"user_id"`
	Type    string    `json:"type"` // e-mail, telegram, webhook
	SentAt  time.Time `json:"sent_at"`
	Payload string    `json:"payload"` // message body
}

type NotificationRepo interface {
	Create(n *Notification) error
	ListByUser(userID int64, limit int) ([]*Notification, error)
}
