package main

import (
	"context"
	"errors"
	"github.com/NordCoder/Pingerus/internal/services/scheduler"
	"github.com/NordCoder/Pingerus/internal/services/scheduler/repo"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "github.com/NordCoder/Pingerus/internal/config/scheduler"
	"github.com/NordCoder/Pingerus/internal/obs"
	kafkaRepo "github.com/NordCoder/Pingerus/internal/repository/kafka"
	pg "github.com/NordCoder/Pingerus/internal/repository/postgres"

	"go.uber.org/zap"
)

func main() {
	// init
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load("../config/scheduler.yaml") // todo path to config to config??
	if err != nil {
		log.Fatal(err)
	}

	// logger
	l, err := obs.NewLogger(cfg.Log.AsLoggerConfig())
	if err != nil {
		log.Fatal(err)
	}
	l.Info("starting scheduler",
		zap.Any("kafka_out", cfg.Kafka),
		zap.String("metrics_addr", cfg.Sched.MetricsAddr),
	)

	// otel
	otelCloser, err := obs.SetupOTel(ctx, cfg.OTEL.AsOTELConfig())
	if err != nil {
		l.Fatal("otel init", zap.Error(err))
	}
	defer func() { err = otelCloser.Shutdown(context.Background()) }()

	// db
	db, err := pg.NewDB(ctx, cfg.DB)
	if err != nil {
		log.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()

	// kafka
	kafkaProd := kafkaRepo.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.Topic) // todo bootstrap
	publisher := kafkaRepo.NewCheckEventsKafka(kafkaProd)
	defer func() { _ = kafkaProd.Close() }()

	// run metrics server
	ms := obs.BootstrapMetricsServer(cfg.Sched.MetricsAddr, func(ctx context.Context) error {
		hctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		return db.Pool.Ping(hctx)
	}, l)

	// wiring
	checkRepo := pg.NewCheckRepo(db)
	uc := scheduler.NewUC(
		repo.CheckRepo{R: checkRepo},
		repo.Events{P: publisher},
	)
	runner := scheduler.New(l, uc, &cfg.Sched)

	// run
	errCh := make(chan error, 1)
	go func() { errCh <- runner.Run(ctx) }()

	l.Info("scheduler started")

	// loop
	select {
	case <-ctx.Done():
	case err = <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			l.Error("runner error", zap.Error(err))
		}
	}

	// graceful shutdown
	shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = ms.Shutdown(shCtx)
	l.Info("bye")
}
