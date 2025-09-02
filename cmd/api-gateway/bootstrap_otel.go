package main

import (
	"context"

	config "github.com/NordCoder/Pingerus/internal/config/api-gateway"
	"github.com/NordCoder/Pingerus/internal/obs"
	"go.uber.org/zap"
)

func initOTel(ctx context.Context, cfg *config.Config, logger *zap.Logger) (func(context.Context) error, error) {
	closer, err := obs.SetupOTel(ctx, obs.OTELConfig{
		Enable:      cfg.OTEL.Enable,
		Endpoint:    cfg.OTEL.OTLPEndpoint,
		ServiceName: cfg.OTEL.ServiceName,
		SampleRatio: cfg.OTEL.SampleRatio,
	})
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) error { return closer.Shutdown(ctx) }, nil
}
