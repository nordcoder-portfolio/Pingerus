package main

import (
	"context"
	"errors"
	"github.com/NordCoder/Pingerus/internal/obs/retry"
	outbox2 "github.com/NordCoder/Pingerus/internal/outbox"
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
	//otelCloser, err := obs.SetupOTel(root, obs.OTELConfig{
	//	Enable:      true,
	//	Endpoint:    "",
	//	ServiceName: "ping-worker",
	//	SampleRatio: 1.0,
	//})
	//if err != nil {
	//	log.Fatal("otel init", zap.Error(err))
	//}
	//defer otelCloser.Shutdown(context.Background())

	db, err := pg.NewDB(root, cfg.DB)
	if err != nil {
		log.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()

	checks := pg.NewCheckRepo(db)
	runs := pg.NewRunRepo(db)

	_ = kafka.EnsureTopic(root, cfg.In.Brokers, kafka.TopicSpec{
		Name:              cfg.In.Topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
		MaxWait:           5 * time.Second,
	}, log)

	cons := kafka.NewConsumer(cfg.In.Brokers, cfg.In.GroupID, cfg.In.Topic)
	defer cons.Close()

	prod := kafka.NewProducer(cfg.Out.Brokers, cfg.Out.Topic).WithLogger(log)
	defer prod.Close()

	events := kafka.NewCheckEventsKafka(prod)

	outboxRepo := pg.NewOutboxRepo(db)
	transactor := pg.NewTransactor(db, log)

	dispatch := outbox2.MakeGlobalOutboxHandler(events, retry.DefaultKafkaPolicy(log))
	outboxRunner := outbox2.NewOutboxRunner( // todo config
		log,
		outboxRepo,
		dispatch,
		20,
		100,
		2*time.Second,
		30*time.Second)

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
		Checks:     workerrepo.CheckRepo{R: checks},
		Runs:       workerrepo.RunRepo{R: runs},
		Outbox:     outboxRepo,
		Transactor: transactor,
		Events:     workerrepo.Events{P: events},
		Clock:      systemClock{},
		HTTP:       pingworker.HTTPPing{Client: httpc, UserAgent: cfg.HTTP.UserAgent},
	}

	ctrl := &pingworker.Controller{Log: log, Sub: cons, UC: uc}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	outboxRunner.Start(ctx)

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
