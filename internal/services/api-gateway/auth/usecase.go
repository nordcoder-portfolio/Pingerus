package auth

import (
	"context"
	"errors"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/domain/auth"
	"github.com/NordCoder/Pingerus/internal/domain/user"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type Config struct {
	Secret     []byte
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	Now        func() time.Time
}

type Usecase struct {
	users user.Repo
	rt    auth.RefreshTokenRepo
	cfg   Config
}

func NewUseCase(users user.Repo, rt auth.RefreshTokenRepo, cfg Config) *Usecase {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Usecase{users: users, rt: rt, cfg: cfg}
}

func (u *Usecase) SignUp(ctx context.Context, email, password string) (*user.User, string, string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", "", fmt.Errorf("hash password: %w", err)
	}
	user := &user.User{Email: email, Password: string(hash)}
	if err := u.users.Create(user); err != nil {
		return nil, "", "", err
	}
	access, refresh, err := u.issueTokens(ctx, user.ID)
	if err != nil {
		return nil, "", "", err
	}
	return user, access, refresh, nil
}

func (u *Usecase) SignIn(ctx context.Context, email, password string) (*user.User, string, string, error) {
	user, err := u.users.GetByEmail(email)
	if err != nil {
		return nil, "", "", ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return nil, "", "", ErrInvalidCredentials
	}
	access, refresh, err := u.issueTokens(ctx, user.ID)
	if err != nil {
		return nil, "", "", err
	}
	return user, access, refresh, nil
}

func (u *Usecase) Refresh(ctx context.Context, raw string) (string, string, int64, error) {
	if raw == "" {
		return "", "", 0, ErrInvalidCredentials
	}
	hash := HashToken(raw)
	rec, err := u.rt.FindValid(ctx, hash)
	if err != nil {
		return "", "", 0, err
	}
	now := u.cfg.Now()
	if rec.ExpiresAt.Before(now) || rec.Revoked {
		return "", "", 0, ErrInvalidCredentials
	}
	if err := u.rt.Revoke(ctx, rec.TokenHash); err != nil {
		return "", "", 0, err
	}
	access, refresh, err := u.issueTokens(ctx, rec.UserID)
	if err != nil {
		return "", "", 0, err
	}
	return access, refresh, rec.UserID, nil
}

func (u *Usecase) Logout(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	return u.rt.Revoke(ctx, HashToken(raw))
}

func (u *Usecase) issueTokens(ctx context.Context, userID int64) (access string, refreshRaw string, err error) {
	now := u.cfg.Now()
	claims := auth.AccessClaims{
		Sub: strconv.FormatInt(userID, 10),
		Iat: now.Unix(),
		Exp: now.Add(u.cfg.AccessTTL).Unix(),
	}
	access, err = SignedString(claims, u.cfg.Secret)
	if err != nil {
		return "", "", fmt.Errorf("sign access: %w", err)
	}
	refreshRaw, err = GenerateRawToken(32)
	if err != nil {
		return "", "", fmt.Errorf("gen refresh: %w", err)
	}
	rec := &auth.RefreshToken{
		UserID:    userID,
		TokenHash: HashToken(refreshRaw),
		IssuedAt:  now,
		ExpiresAt: now.Add(u.cfg.RefreshTTL),
		Revoked:   false,
	}
	if err := u.rt.Create(ctx, rec); err != nil {
		return "", "", fmt.Errorf("save refresh: %w", err)
	}
	return access, refreshRaw, nil
}

func (u *Usecase) ParseAccess(token string) (int64, error) {
	cl, err := ParseAndValidate(token, u.cfg.Secret)
	if err != nil {
		return 0, err
	}
	id, _ := strconv.ParseInt(cl.Sub, 10, 64)
	return id, nil
}
