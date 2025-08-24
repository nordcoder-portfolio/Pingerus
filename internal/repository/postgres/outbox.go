package postgres

import (
	"context"
	"errors"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"time"

	"github.com/NordCoder/Pingerus/internal/domain/outbox"
)

type OutboxRepo struct{ db *DB }

func NewOutboxRepo(db *DB) *OutboxRepo { return &OutboxRepo{db: db} }

const (
	qEnqueue = `
INSERT INTO outbox (idempotency_key, data, status, kind, traceparent, tracestate, baggage)
VALUES ($1, $2, 'CREATED', $3, $4, $5, $6)
ON CONFLICT (idempotency_key) DO NOTHING;`

	qPickLocked = `
WITH cand AS (
  SELECT idempotency_key
  FROM outbox
  WHERE
    status = 'CREATED'
    OR (status = 'IN_PROGRESS' AND updated_at < now() - $2::interval)
  ORDER BY created_at
  FOR UPDATE SKIP LOCKED
  LIMIT $1
), upd AS (
  UPDATE outbox o
  SET status = 'IN_PROGRESS',
      updated_at = now()
  FROM cand
  WHERE o.idempotency_key = cand.idempotency_key
    AND (
      o.status = 'CREATED'
      OR (o.status = 'IN_PROGRESS' AND o.updated_at < now() - $2::interval)
    )
  RETURNING o.idempotency_key, o.kind, o.data, o.status, o.created_at, o.updated_at, o.traceparent, o.tracestate, o.baggage
)
SELECT idempotency_key, kind, data, status, created_at, updated_at, traceparent, tracestate, baggage
FROM upd;`

	qMarkSuccess = `
UPDATE outbox
SET status = 'SUCCESS', updated_at = now()
WHERE idempotency_key = ANY($1)
  AND status = 'IN_PROGRESS';`
)

func (r *OutboxRepo) Enqueue(ctx context.Context, key string, kind outbox.Kind, data []byte) error {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	eq := r.db.execQueryer(ctx)
	_, err := eq.Exec(ctx, qEnqueue, key, data, kind, carrier["traceparent"], carrier["tracestate"], carrier["baggage"])
	return err
}

func (r *OutboxRepo) PickBatch(ctx context.Context, batch int, inProgressTTL time.Duration) ([]outbox.Message, error) {
	if batch <= 0 {
		return nil, errors.New("batch must be > 0")
	}
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	ttl := fmt.Sprintf("%f seconds", inProgressTTL.Seconds())
	rows, err := tx.Query(ctx, qPickLocked, batch, ttl)
	if err != nil {
		return nil, fmt.Errorf("outbox pick: %w", err)
	}
	defer rows.Close()

	var out []outbox.Message
	for rows.Next() {
		var m outbox.Message
		var status string
		if err := rows.Scan(&m.IdempotencyKey, &m.Kind, &m.Data, &status, &m.CreatedAt, &m.UpdatedAt, &m.Traceparent, &m.Tracestate, &m.Baggage); err != nil {
			return nil, fmt.Errorf("outbox scan: %w", err)
		}
		m.Status = outbox.Status(status)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return out, nil
}

func (r *OutboxRepo) MarkSuccess(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	tag, err := r.db.Pool.Exec(ctx, qMarkSuccess, keys)
	if err != nil {
		return fmt.Errorf("outbox mark success: %w", err)
	}
	_ = tag
	return nil
}
