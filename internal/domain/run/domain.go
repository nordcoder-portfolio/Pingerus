package run

import "time"

type Run struct {
	ID        int64     `json:"id"`
	CheckID   int64     `json:"check_id"`
	Timestamp time.Time `json:"timestamp"`
	Status    bool      `json:"status"`
	Code      int       `json:"code"`
	Latency   int64     `json:"latency"`
}
