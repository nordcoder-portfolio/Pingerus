package notification

import "time"

type Notification struct {
	ID      int64     `json:"id"`
	CheckID int64     `json:"check_id"`
	UserID  int64     `json:"user_id"`
	Type    string    `json:"type"`
	SentAt  time.Time `json:"sent_at"`
	Payload string    `json:"payload"`
}
