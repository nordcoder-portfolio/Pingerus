package auth

import (
	"context"
	"time"
)

type AccessClaims struct {
	Sub string `json:"sub"` // user id
	Iat int64  `json:"iat"` // created at
	Exp int64  `json:"exp"` // expires at
}

type RefreshToken struct {
	ID        int64
	UserID    int64
	TokenHash string
	IssuedAt  time.Time
	ExpiresAt time.Time
	Revoked   bool
}

type RefreshTokenRepo interface {
	Create(ctx context.Context, t *RefreshToken) error
	FindValid(ctx context.Context, tokenHash string) (*RefreshToken, error)
	Revoke(ctx context.Context, tokenHash string) error
}
