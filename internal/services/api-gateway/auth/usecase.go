package auth

import (
	"context"
	"errors"
	"fmt"
	domainauth "github.com/NordCoder/Pingerus/internal/domain/auth"
	"github.com/NordCoder/Pingerus/internal/domain/user"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailExists        = errors.New("email already registered")
	ErrWeakPassword       = errors.New("password is too weak")
)

type Config struct {
	Secret     []byte
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	Now        func() time.Time
}

type Usecase struct {
	users user.Repo
	rt    domainauth.RefreshTokenRepo
	cfg   Config
}

func NewUseCase(users user.Repo, rt domainauth.RefreshTokenRepo, cfg Config) *Usecase {
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Usecase{users: users, rt: rt, cfg: cfg}
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func (u *Usecase) SignUp(ctx context.Context, email, password string) (*user.User, string, string, error) {
	email = normalizeEmail(email)
	if len(password) < 8 {
		return nil, "", "", ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", "", fmt.Errorf("hash password: %w", err)
	}
	newUser := &user.User{Email: email, Password: string(hash), CreatedAt: u.cfg.Now(), UpdatedAt: u.cfg.Now()}
	if err := u.users.Create(ctx, newUser); err != nil {
		// маппим нарушение уникальности email (сделай UNIQUE(email) в БД) в ErrEmailExists
		if isUniqueViolation(err) {
			return nil, "", "", ErrEmailExists
		}
		return nil, "", "", err
	}
	access, refresh, err := u.issueTokens(ctx, newUser.ID)
	if err != nil {
		return nil, "", "", err
	}
	return newUser, access, refresh, nil
}

func (u *Usecase) SignIn(ctx context.Context, email, password string) (*user.User, string, string, error) {
	email = normalizeEmail(email)
	uRec, err := u.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", "", ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(uRec.Password), []byte(password)) != nil {
		return nil, "", "", ErrInvalidCredentials
	}
	access, refresh, err := u.issueTokens(ctx, uRec.ID)
	if err != nil {
		return nil, "", "", err
	}
	return uRec, access, refresh, nil
}

func (u *Usecase) Refresh(ctx context.Context, raw string) (string, string, int64, error) {
	if raw == "" {
		return "", "", 0, ErrInvalidCredentials
	}
	hash := HashToken(raw)
	rec, err := u.rt.FindValid(ctx, hash)
	if err != nil {
		return "", "", 0, ErrInvalidCredentials
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
	claims := domainauth.AccessClaims{
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
	rec := &domainauth.RefreshToken{
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
		return 0, ErrInvalidCredentials
	}
	id, err := strconv.ParseInt(cl.Sub, 10, 64)
	if err != nil {
		return 0, ErrInvalidCredentials
	}
	return id, nil
}

// todo make error
func isUniqueViolation(err error) bool { return false }
