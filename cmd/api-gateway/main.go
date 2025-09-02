package main

import (
	"context"
	"errors"
	"github.com/NordCoder/Pingerus/internal/obs"
	pg "github.com/NordCoder/Pingerus/internal/repository/postgres"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "github.com/NordCoder/Pingerus/internal/config/api-gateway"
	"go.uber.org/zap"
)

func main() {
	// init
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// cfg
	cfg, err := config.Load("../config/api-gateway.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// logger
	logger, err := obs.NewLogger(cfg.Log.AsLoggerConfig())
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = logger.Sync() }()
	logger.Info("starting api-gateway", zap.String("env", cfg.App.Env), zap.String("ver", cfg.App.Version))

	// otel
	closer, err := obs.SetupOTel(rootCtx, cfg.OTEL.AsOTELConfig())
	if err != nil {
		logger.Fatal("otel init", zap.Error(err))
	}
	defer func() { _ = closer.Shutdown(rootCtx) }()

	// db
	db, err := pg.NewDB(rootCtx, cfg.DB)
	if err != nil {
		logger.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()

	// grpc
	grpcServer, grpcLn, grpcMetrics, err := buildGRPCServer(cfg, logger, db)
	if err != nil {
		logger.Fatal("build grpc", zap.Error(err))
	}

	grpcErrCh := make(chan error, 1)
	go func() { grpcErrCh <- serveGRPC(grpcServer, grpcLn, cfg, logger) }()

	// http
	httpSrv, conn, err := buildHTTPServer(rootCtx, cfg, logger, db, grpcMetrics)
	if err != nil {
		logger.Fatal("build http server", zap.Error(err))
	}
	defer func() { _ = conn.Close() }()

	httpErrCh := make(chan error, 2)
	go func() {
		logger.Info("http listening", zap.String("addr", cfg.Server.HTTPAddr))
		httpErrCh <- httpSrv.ListenAndServe()
	}()

	// runner
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

	// graceful shutdown
	shCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.GracefulTimeout)
	defer cancel()

	_ = httpSrv.Shutdown(shCtx)
	gracefulStopGRPC(grpcServer)

	time.Sleep(100 * time.Millisecond)
	logger.Info("bye")
}
