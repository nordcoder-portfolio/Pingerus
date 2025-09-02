package main

import (
	"context"

	config "github.com/NordCoder/Pingerus/internal/config/api-gateway"
	pg "github.com/NordCoder/Pingerus/internal/repository/postgres"
	"go.uber.org/zap"
)

type dbHandle = *pg.DB

func initDB(ctx context.Context, cfg *config.Config, logger *zap.Logger) (dbHandle, error) {
	return pg.NewDB(ctx, pg.Config{
		DSN:               cfg.DB.DSN,
		MaxConns:          cfg.DB.MaxConns,
		MinConns:          cfg.DB.MinConns,
		MaxConnLifetime:   cfg.DB.MaxConnLifetime,
		MaxConnIdleTime:   cfg.DB.MaxConnIdleTime,
		HealthCheckPeriod: cfg.DB.HealthCheckPeriod,
		QueryTimeout:      cfg.DB.QueryTimeout,
	})
}
