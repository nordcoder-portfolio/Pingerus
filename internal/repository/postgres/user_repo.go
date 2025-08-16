// internal/repository/postgres/user_repo.go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/domain"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
)

type UserRepo struct {
	db *DB
}

func NewUserRepo(db *DB) *UserRepo { return &UserRepo{db: db} }

const (
	qUserInsert = `
INSERT INTO users(email, password_hash, is_active)
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

	// NULL в $2/$3 означает «не менять поле»
	qUserUpdate = `
UPDATE users
SET email         = COALESCE($2, email),
    password_hash = COALESCE($3, password_hash),
    updated_at    = now()
WHERE id = $1
RETURNING id, email, password_hash, created_at, updated_at;`
)

// Create — создаёт пользователя. Возвращает ErrConflict при UNIQUE(email)
func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	row := r.db.Pool.QueryRow(ctx, qUserInsert, u.Email, u.Password)
	if err := scanUser(row, u); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return ErrConflict
		}
		return err
	}
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	row := r.db.Pool.QueryRow(ctx, qUserByID, id)
	var u domain.User
	if err := scanUser(row, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	row := r.db.Pool.QueryRow(ctx, qUserByEmail, email)
	var u domain.User
	if err := scanUser(row, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// Update — частичное обновление. Передавай nil, если поле менять не нужно.
func (r *UserRepo) Update(ctx context.Context, id int64, newEmail *string, newPasswordHash *string) (*domain.User, error) {
	ctx, cancel := r.db.withTimeout(ctx)
	defer cancel()

	row := r.db.Pool.QueryRow(ctx, qUserUpdate, id, newEmail, newPasswordHash)
	var u domain.User
	if err := scanUser(row, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func scanUser(row pgx.Row, out *domain.User) error {
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
