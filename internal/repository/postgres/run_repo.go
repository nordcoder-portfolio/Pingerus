package postgres

import (
	"context"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/domain/run"
)

var _ run.Repo = (*RunRepoImpl)(nil)

type RunRepoImpl struct{ db *DB }

func NewRunRepo(db *DB) *RunRepoImpl { return &RunRepoImpl{db: db} }

const (
	qRunInsert = `
INSERT INTO runs (check_id, ts, status, latency_ms, code)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;
`
	qRunsByCheck = `
SELECT id, check_id, ts, status, latency_ms, code
FROM runs
WHERE check_id = $1
ORDER BY ts DESC
LIMIT $2;
`
)

func (r *RunRepoImpl) Insert(ctx context.Context, run *run.Run) error {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	q := `
INSERT INTO runs (check_id, ts, status, code, latency_ms)
VALUES ($1,$2,$3,$4,$5)
RETURNING id;`

	eq := r.db.execQueryer(ctx)
	return eq.QueryRow(ctx, q,
		run.CheckID, run.Timestamp, run.Status, run.Code, run.Latency,
	).Scan(&run.ID)
}

func (r *RunRepoImpl) ListByCheck(ctx context.Context, checkID int64, limit int) ([]*run.Run, error) {
	if limit <= 0 {
		limit = 50
	}

	ctx, cancel := r.db.withTimeout(context.Background())
	defer cancel()

	rows, err := r.db.Pool.Query(ctx, qRunsByCheck, checkID, limit)
	if err != nil {
		return nil, fmt.Errorf("query runs: %w", err)
	}
	defer rows.Close()

	out := make([]*run.Run, 0, limit)
	for rows.Next() {
		var rr run.Run
		if err := rows.Scan(&rr.ID, &rr.CheckID, &rr.Timestamp, &rr.Status, &rr.Latency, &rr.Code); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		rp := rr
		out = append(out, &rp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}
