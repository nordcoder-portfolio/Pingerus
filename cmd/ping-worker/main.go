package main

import (
	"context"
	"errors"
	pingworker "github.com/NordCoder/Pingerus/internal/services/ping-worker"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "github.com/NordCoder/Pingerus/internal/config/ping-worker"
	"github.com/NordCoder/Pingerus/internal/obs"
	"github.com/NordCoder/Pingerus/internal/repository/kafka"
	pg "github.com/NordCoder/Pingerus/internal/repository/postgres"
	workerrepo "github.com/NordCoder/Pingerus/internal/services/ping-worker/repo"

	"go.uber.org/zap"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

func main() {
	cfg, err := config.Load("../config/ping-worker.yaml")
	if err != nil {
		panic(err)
	}

	logCfg := zap.NewProductionConfig()
	if cfg.LogLevel == "debug" {
		logCfg = zap.NewDevelopmentConfig()
	}
	log, _ := logCfg.Build()
	defer log.Sync()
	log = log.With(zap.String("service", "ping-worker"))

	root := context.Background()

	db, err := pg.NewDB(root, cfg.DB)
	if err != nil {
		log.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()

	checks := pg.NewCheckRepo(db)
	runs := pg.NewRunRepo(db)

	cons := kafka.NewConsumer(cfg.In.Brokers, cfg.In.GroupID, cfg.In.Topic)
	defer cons.Close()

	prod := kafka.NewProducer(cfg.Out.Brokers, cfg.Out.Topic)
	defer prod.Close()

	events := kafka.NewCheckEventsKafka(prod)

	httpc := pingworker.New(config.HTTPPing{
		Timeout:         cfg.HTTP.Timeout,
		UserAgent:       cfg.HTTP.UserAgent,
		FollowRedirects: cfg.HTTP.FollowRedirects,
		VerifyTLS:       cfg.HTTP.VerifyTLS,
	})

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

	uc := &pingworker.Handler{
		Checks: workerrepo.CheckRepo{R: checks},
		Runs:   workerrepo.RunRepo{R: runs},
		Events: workerrepo.Events{P: events},
		Clock:  systemClock{},
		HTTP:   pingworker.HTTPPing{Client: httpc, UserAgent: cfg.HTTP.UserAgent},
	}

	ctrl := &pingworker.Controller{Log: log, Sub: cons, UC: uc}

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
