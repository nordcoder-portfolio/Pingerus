package auth

import "context"

type RefreshTokenRepo interface {
	Create(ctx context.Context, t *RefreshToken) error
	FindValid(ctx context.Context, tokenHash string) (*RefreshToken, error)
	Revoke(ctx context.Context, tokenHash string) error
}
