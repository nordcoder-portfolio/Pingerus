package domain

import "time"

type Check struct {
	ID         int64         `json:"id"`
	UserID     int64         `json:"user_id"`
	Name       string        `json:"name"`
	URL        string        `json:"url"`
	Interval   time.Duration `json:"interval"`
	Active     bool          `json:"active"`
	LastStatus *bool         `json:"last_status"`
	NextRun    time.Time     `json:"next_run"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

type CheckRepo interface {
	Create(c *Check) error
	GetByID(id int64) (*Check, error)
	ListByUser(userID int64) ([]*Check, error)
	Update(c *Check) error
	Delete(id int64) error
	FetchDue(limit int) ([]*Check, error)
}
