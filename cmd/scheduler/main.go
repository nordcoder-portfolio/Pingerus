package main

import (
	"context"
	"errors"
	config "github.com/NordCoder/Pingerus/internal/config/scheduler"
	"github.com/NordCoder/Pingerus/internal/obs"
	"github.com/NordCoder/Pingerus/internal/repository/kafka"
	service "github.com/NordCoder/Pingerus/internal/services/scheduler"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	pg "github.com/NordCoder/Pingerus/internal/repository/postgres"
)

func main() {
	cfg, err := config.Load("../../config/scheduler.yaml")
	if err != nil {
		panic(err)
	}

	logCfg := zap.NewProductionConfig()
	if cfg.LogLevel == "debug" {
		logCfg = zap.NewDevelopmentConfig()
	}
	log, _ := logCfg.Build()
	defer log.Sync()
	log = log.With(zap.String("service", "scheduler"))

	ctx := context.Background()

	db, err := pg.NewDB(ctx, cfg.DB)
	if err != nil {
		log.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()

	checkRepo := pg.NewCheckRepo(db)

	kafkaProd := kafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.Topic)
	publisher := kafka.NewCheckEventsKafka(kafkaProd)
	defer kafkaProd.Close()

	ms := obs.CreateMetricsServer(cfg.Sched.MetricsAddr, func(ctx context.Context) error {
		hctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		return db.Pool.Ping(hctx)
	})
	go func() {
		log.Info("metrics listening", zap.String("addr", cfg.Sched.MetricsAddr))
		if err := ms.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("metrics server error", zap.Error(err))
		}
	}()

	runner := service.NewRunner(log, checkRepo, publisher, cfg.Sched)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- runner.Run(ctx) }()

	select {
	case <-ctx.Done():
	case err = <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error("runner error", zap.Error(err))
		}
	}

	shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = ms.Shutdown(shCtx)
	log.Info("bye")
}
