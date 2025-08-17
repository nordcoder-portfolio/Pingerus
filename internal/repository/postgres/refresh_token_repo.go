package postgres

import (
	"context"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/domain/auth"
)

type RefreshTokenRepo struct{ db *DB }

func NewRefreshTokenRepo(db *DB) *RefreshTokenRepo { return &RefreshTokenRepo{db: db} }

const (
	qRTCreate = `
INSERT INTO refresh_tokens(user_id, token_hash, issued_at, expires_at, revoked)
VALUES ($1, $2, $3, $4, FALSE)
RETURNING id;
`
	qRTFindValid = `
SELECT id, user_id, token_hash, issued_at, expires_at, revoked
FROM refresh_tokens
WHERE token_hash = $1 AND revoked = FALSE AND expires_at > NOW()
LIMIT 1;
`
	qRTRevoke = `
UPDATE refresh_tokens SET revoked=TRUE WHERE token_hash = $1;
`
)

func (r *RefreshTokenRepo) Create(ctx context.Context, t *auth.RefreshToken) error {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	return r.db.Pool.QueryRow(ctx, qRTCreate, t.UserID, t.TokenHash, t.IssuedAt, t.ExpiresAt).Scan(&t.ID)
}

func (r *RefreshTokenRepo) FindValid(ctx context.Context, tokenHash string) (*auth.RefreshToken, error) {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	var t auth.RefreshToken
	if err := r.db.Pool.QueryRow(ctx, qRTFindValid, tokenHash).
		Scan(&t.ID, &t.UserID, &t.TokenHash, &t.IssuedAt, &t.ExpiresAt, &t.Revoked); err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find valid refresh: %w", err)
	}
	return &t, nil
}

func (r *RefreshTokenRepo) Revoke(ctx context.Context, tokenHash string) error {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	_, err := r.db.Pool.Exec(ctx, qRTRevoke, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke refresh: %w", err)
	}
	return nil
}
