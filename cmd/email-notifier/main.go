package main

import (
	"context"
	"errors"
	"net/http"
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

func main() {
	// ── Load config
	cfg, err := config.Load("../config/email-notifier.yaml")
	if err != nil {
		panic(err)
	}

	// ── Logger
	logCfg := zap.NewProductionConfig()
	if cfg.LogLevel == "debug" {
		logCfg = zap.NewDevelopmentConfig()
	}
	log, _ := logCfg.Build()
	defer log.Sync()
	log = log.With(zap.String("service", "email-notifier"))

	log.Info("starting email-notifier",
		zap.Any("kafka_in", cfg.In),
		zap.String("metrics_addr", cfg.Server.MetricsAddr),
		zap.String("smtp_addr", cfg.SMTP.Addr),
	)

	rootCtx := context.Background()
	otelCloser, err := obs.SetupOTel(rootCtx, obs.OTELConfig{
		Enable:      false,
		Endpoint:    "",
		ServiceName: "email-notifier",
		SampleRatio: 1.0,
	})
	if err != nil {
		log.Warn("otel init", zap.Error(err))
	}
	defer otelCloser.Shutdown(context.Background())

	// ── DB
	db, err := pg.NewDB(rootCtx, cfg.DB)
	if err != nil {
		log.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()
	log.Info("db connected")

	// ── Repos
	checks := pg.NewCheckRepo(db)
	users := pg.NewUserRepo(db)
	notifs := pg.NewNotificationRepo(db)

	// ── Kafka consumer
	cons := kafka.NewConsumer(cfg.In.Brokers, cfg.In.GroupID, cfg.In.Topic).WithLogger(log)
	defer cons.Close()
	log.Info("kafka consumer initialized",
		zap.Strings("brokers", cfg.In.Brokers),
		zap.String("group_id", cfg.In.GroupID),
		zap.String("topic", cfg.In.Topic),
	)

	// ── Metrics/health server
	ms := obs.CreateMetricsServer(cfg.Server.MetricsAddr, func(ctx context.Context) error {
		hctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		return db.Pool.Ping(hctx)
	})
	go func() {
		log.Info("metrics listening", zap.String("addr", cfg.Server.MetricsAddr))
		if err := ms.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("metrics server error", zap.Error(err))
		}
	}()

	// ── Mailer & usecase
	mailer := notifier.New(cfg.SMTP).WithLogger(log)
	uc := &notifier.Handler{
		Checks: repo.CheckReader{R: checks},
		Users:  repo.UserReader{R: users},
		Store:  repo.NotificationRepo{R: notifs},
		Out:    mailer,
		Clock:  systemClock{},
		Log:    log,
	}

	ctrl := &notifier.Controller{Log: log, Sub: cons, UC: uc}

	// ── Run
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("controller starting")
		errCh <- ctrl.Run(ctx)
	}()

	var runErr error
	select {
	case <-ctx.Done():
		log.Info("shutdown signal")
	case runErr = <-errCh:
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
			log.Error("controller error", zap.Error(runErr))
		}
	}

	// ── Shutdown
	shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = ms.Shutdown(shCtx)
	log.Info("bye")
}
