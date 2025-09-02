package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "github.com/NordCoder/Pingerus/internal/config/email-notifier"
	"github.com/NordCoder/Pingerus/internal/obs"
	"github.com/NordCoder/Pingerus/internal/repository/kafka"
	pg "github.com/NordCoder/Pingerus/internal/repository/postgres"
	notifier "github.com/NordCoder/Pingerus/internal/services/email-notifier"
	"github.com/NordCoder/Pingerus/internal/services/email-notifier/repo"

	"go.uber.org/zap"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

func wiring(db *pg.DB, cfg *config.Config, cons *kafka.Consumer, l *zap.Logger) *notifier.Controller {
	checks := pg.NewCheckRepo(db)
	users := pg.NewUserRepo(db)
	notifs := pg.NewNotificationRepo(db)
	mailer := notifier.New(cfg.SMTP).WithLogger(l)

	uc := &notifier.Handler{
		Checks: repo.CheckReader{R: checks},
		Users:  repo.UserReader{R: users},
		Store:  repo.NotificationRepo{R: notifs},
		Out:    mailer,
		Clock:  systemClock{},
		Log:    l,
	}

	return &notifier.Controller{Log: l, Sub: cons, UC: uc}
}

func main() {
	// init
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load("../config/email-notifier.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// logger
	l, err := obs.NewLogger(cfg.Log.AsLoggerConfig())
	if err != nil {
		log.Fatal(err)
	}

	l.Info("starting email-notifier",
		zap.Any("kafka_in", cfg.In),
		zap.String("metrics_addr", cfg.Server.MetricsAddr),
		zap.String("smtp_addr", cfg.SMTP.Addr),
	)

	// otel
	otelCloser, err := obs.SetupOTel(rootCtx, cfg.OTEL.AsOTELConfig())
	if err != nil {
		l.Warn("otel init", zap.Error(err))
	}
	defer func() { _ = otelCloser.Shutdown(context.Background()) }()

	// db
	db, err := pg.NewDB(rootCtx, cfg.DB)
	if err != nil {
		l.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()
	l.Info("db connected")

	// metrics
	ms := obs.BootstrapMetricsServer(cfg.Server.MetricsAddr, func(ctx context.Context) error {
		hctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		return db.Pool.Ping(hctx)
	}, l)

	// kafka
	cons := kafka.BootstrapConsumer(rootCtx, cfg.In.AsConsumerConfig(), l).WithLogger(l)
	defer func() { _ = cons.Close() }()
	l.Info("kafka consumer initialized",
		zap.Strings("brokers", cfg.In.Brokers),
		zap.String("group_id", cfg.In.GroupID),
		zap.String("topic", cfg.In.Topic),
	)

	// start
	ctrl := wiring(db, cfg, cons, l)
	errCh := make(chan error, 1)
	go func() {
		l.Info("controller starting")
		errCh <- ctrl.Run(rootCtx)
	}()

	// main loop
	var runErr error
	select {
	case <-rootCtx.Done():
		l.Info("shutdown signal")
	case runErr = <-errCh:
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
			l.Error("controller error", zap.Error(runErr))
		}
	}

	// graceful metrics server shutdown
	shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = ms.Shutdown(shCtx)
	l.Info("bye")
}
