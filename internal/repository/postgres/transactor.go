package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"

	"go.uber.org/zap"

	"github.com/jackc/pgx/v5"
)

type Transactor interface {
	WithTx(ctx context.Context, function func(ctx context.Context) error) error
}

var _ Transactor = (*transactorImpl)(nil)

type transactorImpl struct {
	db     *DB
	logger *zap.Logger
}

func NewTransactor(db *DB, logger *zap.Logger) *transactorImpl {
	return &transactorImpl{
		db:     db,
		logger: logger,
	}
}
func (t *transactorImpl) WithTx(ctx context.Context, function func(ctx context.Context) error) (txErr error) {
	ctxWithTx, tx, err := injectTx(ctx, t.db)

	if err != nil {
		return fmt.Errorf("can not inject transaction, error: %w", err)
	}

	defer func() {
		if txErr != nil {
			err = tx.Rollback(ctxWithTx)
			if err != nil {
				t.logger.Error("rollback", zap.Error(err))
			}
			return
		}

		err = tx.Commit(ctxWithTx)
		if err != nil {
			t.logger.Error("commit", zap.Error(err))
		}
	}()

	err = function(ctxWithTx)

	if err != nil {
		return fmt.Errorf("function execution error: %w", err)
	}

	return nil
}

type txInjector struct{}

var ErrTxNotFound = errors.New("tx not found in context")

func injectTx(ctx context.Context, pool *DB) (context.Context, pgx.Tx, error) {
	if tx, err := extractTx(ctx); err == nil {
		return ctx, tx, nil
	}

	tx, err := pool.Pool.Begin(ctx)

	if err != nil {
		return nil, nil, err
	}

	return context.WithValue(ctx, txInjector{}, tx), tx, nil
}

func extractTx(ctx context.Context) (pgx.Tx, error) {
	tx, ok := ctx.Value(txInjector{}).(pgx.Tx)

	if !ok {
		return nil, ErrTxNotFound
	}

	return tx, nil
}

type execQueryer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (db *DB) execQueryer(ctx context.Context) execQueryer {
	if tx, err := extractTx(ctx); err == nil && tx != nil {
		return tx
	}
	return db.Pool
}
