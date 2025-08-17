package main

import (
	"context"
	"errors"
	config "github.com/NordCoder/Pingerus/internal/config/email-notifier"
	"github.com/NordCoder/Pingerus/internal/obs"
	"github.com/NordCoder/Pingerus/internal/repository/kafka"
	notifier "github.com/NordCoder/Pingerus/internal/services/email-notifier"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	pg "github.com/NordCoder/Pingerus/internal/repository/postgres" // NewDB, Config
)

func main() {
	cfg, err := config.Load("config/email-notifier.yaml")
	if err != nil {
		panic(err)
	}

	logCfg := zap.NewProductionConfig()
	if cfg.LogLevel == "debug" {
		logCfg = zap.NewDevelopmentConfig()
	}
	log, _ := logCfg.Build()
	defer log.Sync()
	log = log.With(zap.String("service", "email-notifier"))

	ctx := context.Background()

	db, err := pg.NewDB(ctx, cfg.DB)
	if err != nil {
		log.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()

	checks := pg.NewCheckRepo(db)
	var users = pg.NewUserRepo(db)
	var notifs = pg.NewNotificationRepo(db)

	cons := kafka.NewConsumer(cfg.In.Brokers, cfg.In.GroupID, cfg.In.Topic)
	defer cons.Close()

	mailer := notifier.NewMailer(cfg.SMTP)

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

	runner := notifier.NewRunner(log, cons, mailer, checks, users, notifs)

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
