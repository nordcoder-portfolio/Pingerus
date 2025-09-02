package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "github.com/NordCoder/Pingerus/internal/config/api-gateway"
	"go.uber.org/zap"
)

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load("../config/api-gateway.yaml")
	if err != nil {
		panic(err)
	}

	logger, err := initLogger(cfg)
	if err != nil {
		panic(err)
	}
	defer func() { _ = logger.Sync() }()
	logger.Info("starting api-gateway", zap.String("env", cfg.App.Env), zap.String("ver", cfg.App.Version))

	otelShutdown, err := initOTel(rootCtx, cfg, logger)
	if err != nil {
		logger.Fatal("otel init", zap.Error(err))
	}
	defer func() { _ = otelShutdown(rootCtx) }()

	db, err := initDB(rootCtx, cfg, logger)
	if err != nil {
		logger.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()

	grpcServer, grpcLn, grpcMetrics, err := buildGRPCServer(cfg, logger, db)
	if err != nil {
		logger.Fatal("build grpc", zap.Error(err))
	}

	grpcErrCh := make(chan error, 1)
	go func() { grpcErrCh <- serveGRPC(grpcServer, grpcLn, cfg, logger) }()

	httpSrv, err := buildHTTPServer(rootCtx, cfg, logger, db, grpcMetrics)
	if err != nil {
		logger.Fatal("build http", zap.Error(err))
	}

	httpErrCh := make(chan error, 1)
	go func() { httpErrCh <- serveHTTP(httpSrv, cfg, logger) }()

	var runErr error
	select {
	case <-rootCtx.Done():
		logger.Info("shutdown signal", zap.String("reason", "context canceled"))
	case runErr = <-grpcErrCh:
		if runErr != nil {
			logger.Error("grpc serve", zap.Error(runErr))
		}
	case runErr = <-httpErrCh:
		if runErr != nil && !errors.Is(runErr, http.ErrServerClosed) {
			logger.Error("http serve", zap.Error(runErr))
		}
	}

	shCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.GracefulTimeout)
	defer cancel()

	_ = httpSrv.Shutdown(shCtx)
	gracefulStopGRPC(grpcServer)

	time.Sleep(100 * time.Millisecond)
	logger.Info("bye")
}
