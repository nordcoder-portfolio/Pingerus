package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/domain/check"
	"time"

	"github.com/jackc/pgx/v5"
)

var _ check.Repo = (*CheckRepoImpl)(nil)

type CheckRepoImpl struct {
	db *DB
}

func NewCheckRepo(db *DB) *CheckRepoImpl { return &CheckRepoImpl{db: db} }

const (
	qInsert = `
INSERT INTO checks (user_id, host, interval_sec, active, next_run)
VALUES ($1, $2, $3, TRUE, NOW())
RETURNING id, user_id, host, interval_sec, last_status, next_run, created_at, updated_at, active;
`

	qGetByID = `
SELECT id, user_id, host, interval_sec, last_status, next_run, created_at, updated_at, active
FROM checks
WHERE id = $1;
`

	qListByUser = `
SELECT id, user_id, host, interval_sec, last_status, next_run, created_at, updated_at, active
FROM checks
WHERE user_id = $1
ORDER BY id DESC;
`

	qDelete = `DELETE FROM checks WHERE id = $1;`

	qFetchDue = `
SELECT id, user_id, host, interval_sec, last_status, next_run, created_at, updated_at, active
FROM checks
WHERE active = TRUE AND next_run <= NOW()
ORDER BY next_run
FOR UPDATE SKIP LOCKED
LIMIT $1;
`

	qBumpNextRun = `
UPDATE checks
SET next_run = NOW() + (interval_sec * INTERVAL '1 second'),
    updated_at = NOW()
WHERE id = ANY($1);
`
)

func scanFull(row pgx.Row, c *check.Check) error {
	var (
		intervalSec int
	)
	if err := row.Scan(
		&c.ID,
		&c.UserID,
		&c.URL,
		&intervalSec,
		&c.LastStatus,
		&c.NextRun,
		&c.CreatedAt,
		&c.UpdatedAt,
		&c.Active,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("scan check: %w", err)
	}
	c.Interval = time.Duration(intervalSec) * time.Second
	c.Name = ""
	return nil
}

func (r *CheckRepoImpl) Create(ctx context.Context, c *check.Check) error {
	ctx, cancel := r.db.withTimeout(context.Background())
	defer cancel()

	intervalSec := int(c.Interval / time.Second)
	if intervalSec < 0 {
		intervalSec = 0
	}

	row := r.db.Pool.QueryRow(ctx, qInsert, c.UserID, c.URL, intervalSec)
	return scanFull(row, c)
}

func (r *CheckRepoImpl) GetByID(ctx context.Context, id int64) (*check.Check, error) {
	ctx, cancel := r.db.withTimeout(context.Background())
	defer cancel()

	var c check.Check
	if err := scanFull(r.db.Pool.QueryRow(ctx, qGetByID, id), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CheckRepoImpl) ListByUser(ctx context.Context, userID int64) ([]*check.Check, error) {
	ctx, cancel := r.db.withTimeout(context.Background())
	defer cancel()

	rows, err := r.db.Pool.Query(ctx, qListByUser, userID)
	if err != nil {
		return nil, fmt.Errorf("query checks: %w", err)
	}
	defer rows.Close()

	var out []*check.Check
	for rows.Next() {
		var c check.Check
		if err := scanFull(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

func (r *CheckRepoImpl) Update(ctx context.Context, c *check.Check) error {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	q := `
UPDATE checks
SET last_status = $2, updated_at = now()
WHERE id = $1;`

	eq := r.db.execQueryer(ctx)
	_, err := eq.Exec(ctx, q, c.ID, c.LastStatus)
	return err
}

func (r *CheckRepoImpl) Delete(ctx context.Context, id int64) error {
	ctx, cancel := r.db.withTimeout(context.Background())
	defer cancel()

	cmd, err := r.db.Pool.Exec(ctx, qDelete, id)
	if err != nil {
		return fmt.Errorf("delete check: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *CheckRepoImpl) FetchDue(ctx context.Context, limit int) ([]*check.Check, error) {
	if limit <= 0 {
		limit = 100
	}
	ctx, cancel := r.db.withTimeout(context.Background())
	defer cancel()

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, qFetchDue, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch due: %w", err)
	}
	defer rows.Close()

	var (
		out []*check.Check
		ids []int64
	)
	for rows.Next() {
		var c check.Check
		if err := scanFull(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, &c)
		ids = append(ids, c.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	if _, err := tx.Exec(ctx, qBumpNextRun, ids); err != nil {
		return nil, fmt.Errorf("bump next_run: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return out, nil
}
