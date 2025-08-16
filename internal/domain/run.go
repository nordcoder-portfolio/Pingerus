package domain

import "time"

type Run struct {
	ID        int64     `json:"id"`
	CheckID   int64     `json:"check_id"`
	Timestamp time.Time `json:"timestamp"`
	Status    bool      `json:"status"`  // true=UP, false=DOWN
	Code      int       `json:"code"`    // HTTP status or ICMP result
	Latency   int64     `json:"latency"` // in ms
}

type RunRepo interface {
	Insert(r *Run) error
	ListByCheck(checkID int64, limit int) ([]*Run, error)
}
