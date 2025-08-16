package postgres

import (
	"context"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/domain"
)

type RunRepo struct{ db *DB }

func NewRunRepo(db *DB) *RunRepo { return &RunRepo{db: db} }

const qRunInsert = `
INSERT INTO runs(check_id, ts, status, latency_ms, code)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;
`

func (r *RunRepo) Insert(ctx context.Context, run domain.Run) (int64, error) {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	var id int64
	if err := r.db.Pool.QueryRow(ctx, qRunInsert, run.CheckID, run.Timestamp, run.Status, run.Latency, run.Code).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert run: %w", err)
	}
	return id, nil
}

func (r *RunRepo) LastByCheck(ctx context.Context, checkID int64, limit int) ([]domain.Run, error) {
	q := `
SELECT id, check_id, ts, status, latency_ms, code
FROM runs
WHERE check_id=$1
ORDER BY ts DESC
LIMIT $2;
`
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	rows, err := r.db.Pool.Query(ctx, q, checkID, limit)
	if err != nil {
		return nil, fmt.Errorf("query runs: %w", err)
	}
	defer rows.Close()

	var res []domain.Run
	for rows.Next() {
		var rr domain.Run
		if err := rows.Scan(&rr.ID, &rr.CheckID, &rr.Timestamp, &rr.Status, &rr.Latency, &rr.Code); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		res = append(res, rr)
	}
	return res, nil
}
