package postgres

import (
	"context"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/domain/notification"
	"time"
)

var _ notification.Repo = (*NotificationRepoImpl)(nil)

type NotificationRepoImpl struct{ db *DB }

func NewNotificationRepo(db *DB) *NotificationRepoImpl { return &NotificationRepoImpl{db: db} }

const (
	qNotifInsert = `
INSERT INTO notifications (check_id, user_id, type, sent_at, payload)
VALUES ($1, $2, $3, COALESCE($4, now()), $5)
RETURNING id, sent_at;
`
	qNotifByUser = `
SELECT id, check_id, user_id, type, sent_at, payload
FROM notifications
WHERE user_id = $1
ORDER BY sent_at DESC
LIMIT $2;
`
)

func (r *NotificationRepoImpl) Create(ctx context.Context, n *notification.Notification) error {
	ctx, cancel := r.db.withTimeout(context.Background())
	defer cancel()

	if err := r.db.Pool.QueryRow(ctx, qNotifInsert,
		n.CheckID,
		n.UserID,
		n.Type,
		nullTime(n.SentAt),
		n.Payload,
	).Scan(&n.ID, &n.SentAt); err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

func (r *NotificationRepoImpl) ListByUser(ctx context.Context, userID int64, limit int) ([]*notification.Notification, error) {
	if limit <= 0 {
		limit = 50
	}

	ctx, cancel := r.db.withTimeout(context.Background())
	defer cancel()

	rows, err := r.db.Pool.Query(ctx, qNotifByUser, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	out := make([]*notification.Notification, 0, limit)
	for rows.Next() {
		var n notification.Notification
		if err := rows.Scan(&n.ID, &n.CheckID, &n.UserID, &n.Type, &n.SentAt, &n.Payload); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		nc := n
		out = append(out, &nc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
