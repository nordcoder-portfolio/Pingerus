package postgres

import (
	"errors"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrConstraint   = errors.New("constraint violation")
	ErrUnauthorized = errors.New("unauthorized")
)

func oneRow(rows pgx.Rows) bool {
	defer rows.Close()
	return rows.Next()
}
