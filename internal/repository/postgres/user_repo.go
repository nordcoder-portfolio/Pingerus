package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/domain/user"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
)

var _ user.Repo = (*UserRepo)(nil)

type UserRepo struct {
	db *DB
}

func NewUserRepo(db *DB) *UserRepo { return &UserRepo{db: db} }

const (
	qUserInsert = `
INSERT INTO users (email, password_hash, is_active)
VALUES ($1, $2, TRUE)
RETURNING id, email, password_hash, created_at, updated_at;`

	qUserByID = `
SELECT id, email, password_hash, created_at, updated_at
FROM users
WHERE id = $1;`

	qUserByEmail = `
SELECT id, email, password_hash, created_at, updated_at
FROM users
WHERE email = $1;`

	qUserUpdate = `
UPDATE users
SET email         = $2,
    password_hash = $3,
    updated_at    = NOW()
WHERE id = $1
RETURNING id, email, password_hash, created_at, updated_at;`
)

func (r *UserRepo) Create(ctx context.Context, u *user.User) error {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	if err := r.db.Pool.QueryRow(ctx, qUserInsert, u.Email, u.Password).
		Scan(&u.ID, &u.Email, &u.Password, &u.CreatedAt, &u.UpdatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("user insert: %w", err)
	}
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*user.User, error) {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	var u user.User
	if err := scanUser(r.db.Pool.QueryRow(ctx, qUserByID, id), &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	var u user.User
	if err := scanUser(r.db.Pool.QueryRow(ctx, qUserByEmail, email), &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) Update(ctx context.Context, u *user.User) error {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	if err := r.db.Pool.QueryRow(ctx, qUserUpdate, u.ID, u.Email, u.Password).
		Scan(&u.ID, &u.Email, &u.Password, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return fmt.Errorf("user update: %w", err)
	}
	return nil
}

func scanUser(row pgx.Row, out *user.User) error {
	var created, updated time.Time
	if err := row.Scan(&out.ID, &out.Email, &out.Password, &created, &updated); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("scan user: %w", err)
	}
	out.CreatedAt = created
	out.UpdatedAt = updated
	return nil
}
