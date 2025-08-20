package auth

import (
	"context"
	"github.com/NordCoder/Pingerus/internal/domain/user"
)

type Usecase interface {
	SignUp(ctx context.Context, email, password string) (*user.User, string, string, error)
	SignIn(ctx context.Context, email, password string) (*user.User, string, string, error)
	Refresh(ctx context.Context, raw string) (string, string, int64, error)
	Logout(ctx context.Context, raw string) error
	ParseAccess(token string) (int64, error)
}

type RefreshTokenRepo interface {
	Create(ctx context.Context, t *RefreshToken) error
	FindValid(ctx context.Context, tokenHash string) (*RefreshToken, error)
	Revoke(ctx context.Context, tokenHash string) error
}
