package main

import (
	"context"
	"errors"
	notifier "github.com/NordCoder/Pingerus/internal/services/email-notifier"
	"github.com/NordCoder/Pingerus/internal/services/email-notifier/repo"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "github.com/NordCoder/Pingerus/internal/config/email-notifier"
	"github.com/NordCoder/Pingerus/internal/obs"
	"github.com/NordCoder/Pingerus/internal/repository/kafka"
	pg "github.com/NordCoder/Pingerus/internal/repository/postgres"

	"go.uber.org/zap"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

func main() {
	cfg, err := config.Load("../config/email-notifier.yaml")
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

	rootCtx := context.Background()

	db, err := pg.NewDB(rootCtx, cfg.DB)
	if err != nil {
		log.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()

	checks := pg.NewCheckRepo(db)
	users := pg.NewUserRepo(db)
	notifs := pg.NewNotificationRepo(db)

	cons := kafka.NewConsumer(cfg.In.Brokers, cfg.In.GroupID, cfg.In.Topic)
	defer cons.Close()

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

	mailer := notifier.New(config.SMTP{
		Addr:       cfg.SMTP.Addr,
		User:       cfg.SMTP.User,
		Password:   cfg.SMTP.Password,
		UseTLS:     cfg.SMTP.UseTLS,
		Timeout:    cfg.SMTP.Timeout,
		From:       cfg.SMTP.From,
		SubjPrefix: cfg.SMTP.SubjPrefix,
	})

	uc := &notifier.Handler{
		Checks: repo.CheckReader{R: checks},
		Users:  repo.UserReader{R: users},
		Store:  repo.NotificationRepo{R: notifs},
		Out:    mailer,
		Clock:  systemClock{},
	}

	ctrl := &notifier.Controller{Log: log, Sub: cons, UC: uc}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- ctrl.Run(ctx) }()

	select {
	case <-ctx.Done():
	case err = <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error("controller error", zap.Error(err))
		}
	}

	shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = ms.Shutdown(shCtx)
	log.Info("bye")
}
