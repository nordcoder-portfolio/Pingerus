package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/domain"
	"time"

	"github.com/jackc/pgx/v5"
)

type CheckRepo struct {
	db *DB
}

func NewCheckRepo(db *DB) *CheckRepo { return &CheckRepo{db: db} }

const (
	qCheckInsert = `
INSERT INTO checks(user_id, host, interval_sec, active, next_run)
VALUES ($1, $2, $3, TRUE, NOW())
RETURNING id, user_id, host, interval_sec, last_status, next_run, created_at, updated_at;
`
	qCheckListByUser = `
SELECT id, user_id, host, interval_sec, last_status, next_run, created_at, updated_at, active
FROM checks WHERE user_id = $1 ORDER BY id DESC;
`
	qCheckDelete = `DELETE FROM checks WHERE id = $1 AND user_id = $2;`

	qFetchDue = `
SELECT id, user_id, host, interval_sec, last_status, next_run
FROM checks
WHERE active = TRUE AND next_run <= NOW()
ORDER BY next_run
FOR UPDATE SKIP LOCKED
LIMIT $1;
`
	qUpdateNextRun = `
UPDATE checks
SET next_run = $2, updated_at = NOW()
WHERE id = $1;
`
	qUpdateLastStatus = `
UPDATE checks SET last_status=$2, updated_at=NOW() WHERE id=$1;
`
)

func (r *CheckRepo) Create(ctx context.Context, c *domain.Check) error {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	row := r.db.Pool.QueryRow(ctx, qCheckInsert, c.UserID, c.URL, c.Interval.Seconds())
	return scanCheck(row, c)
}

func (r *CheckRepo) ListByUser(ctx context.Context, userID int64) ([]domain.Check, error) {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	rows, err := r.db.Pool.Query(ctx, qCheckListByUser, userID)
	if err != nil {
		return nil, fmt.Errorf("query checks: %w", err)
	}
	defer rows.Close()

	var out []domain.Check
	for rows.Next() {
		var c domain.Check
		var created, updated time.Time
		var intervalSec int
		if err := rows.Scan(&c.ID, &c.UserID, &c.URL, &intervalSec, &c.LastStatus, &c.NextRun, &created, &updated, &c.Active); err != nil {
			return nil, fmt.Errorf("scan check: %w", err)
		}
		c.CreatedAt, c.UpdatedAt, c.Interval = created, updated, time.Duration(intervalSec)*time.Second
		out = append(out, c)
	}
	return out, nil
}

func (r *CheckRepo) Delete(ctx context.Context, id, userID int64) error {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()
	cmd, err := r.db.Pool.Exec(ctx, qCheckDelete, id, userID)
	if err != nil {
		return fmt.Errorf("delete check: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *CheckRepo) FetchAndScheduleNext(ctx context.Context, limit int, jitterPct int) ([]domain.Check, error) {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, qFetchDue, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch due: %w", err)
	}
	defer rows.Close()

	var out []domain.Check
	now := time.Now()
	for rows.Next() {
		var c domain.Check
		var intervalSec int
		if err := rows.Scan(&c.ID, &c.UserID, &c.URL, &intervalSec, &c.LastStatus, &c.NextRun); err != nil {
			return nil, fmt.Errorf("scan due: %w", err)
		}

		c.Interval = time.Duration(intervalSec) * time.Second

		next := nextRunWithJitter(now, c.Interval, jitterPct)
		if _, err := tx.Exec(ctx, qUpdateNextRun, c.ID, next); err != nil {
			return nil, fmt.Errorf("update next_run: %w", err)
		}
		out = append(out, c)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return out, nil
}

func (r *CheckRepo) UpdateLastStatus(ctx context.Context, checkID int64, status *bool) error {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()
	_, err := r.db.Pool.Exec(ctx, qUpdateLastStatus, checkID, status)
	if err != nil {
		return fmt.Errorf("update last_status: %w", err)
	}
	return nil
}

func scanCheck(row pgx.Row, c *domain.Check) error {
	var created, updated time.Time
	var intervalSec int
	if err := row.Scan(&c.ID, &c.UserID, &c.URL, &intervalSec, &c.LastStatus, &c.NextRun, &created, &updated); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("scan check: %w", err)
	}
	c.CreatedAt, c.UpdatedAt = created, updated
	return nil
}

func nextRunWithJitter(base time.Time, interval time.Duration, jitterPct int) time.Time {
	if jitterPct <= 0 {
		return base.Add(interval)
	}
	j := int64(interval) * int64(jitterPct) / 100
	n := time.Now().UnixNano()
	off := (n % (2*j + 1)) - j
	return base.Add(time.Duration(int64(interval) + off))
}
