package main

import (
	"context"
	"errors"
	"github.com/NordCoder/Pingerus/internal/obs/retry"
	"github.com/NordCoder/Pingerus/internal/outbox"
	pingworker "github.com/NordCoder/Pingerus/internal/services/ping-worker"
	"log"
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

func wire(cfg *config.Config, db *pg.DB, events *kafka.CheckEventsKafka, cons *kafka.Consumer, l *zap.Logger) (*outbox.Runner, *pingworker.Controller) {
	outboxRepo := pg.NewOutboxRepo(db)
	transactor := pg.NewTransactor(db, l)

	dispatch := outbox.MakeGlobalOutboxHandler(events, retry.DefaultKafkaPolicy(l))
	outboxRunner := outbox.NewOutboxRunner( // todo config
		l,
		outboxRepo,
		dispatch,
		20,
		100,
		2*time.Second,
		30*time.Second)

	checks := pg.NewCheckRepo(db)
	runs := pg.NewRunRepo(db)

	httpc := pingworker.New(config.HTTPPing{
		Timeout:         cfg.HTTP.Timeout,
		UserAgent:       cfg.HTTP.UserAgent,
		FollowRedirects: cfg.HTTP.FollowRedirects,
		VerifyTLS:       cfg.HTTP.VerifyTLS,
	})

	uc := &pingworker.Handler{
		Checks:     workerrepo.CheckRepo{R: checks},
		Runs:       workerrepo.RunRepo{R: runs},
		Outbox:     outboxRepo,
		Transactor: transactor,
		Events:     workerrepo.Events{P: events},
		Clock:      systemClock{},
		HTTP:       pingworker.HTTPPing{Client: httpc, UserAgent: cfg.HTTP.UserAgent},
	}

	return outboxRunner, &pingworker.Controller{Log: l, Sub: cons, UC: uc}
}

func main() {
	// init
	root, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load("../config/ping-worker.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// logger
	l, err := obs.NewLogger(cfg.Log.AsLoggerConfig())
	if err != nil {
		log.Fatal(err)
	}

	// otel
	otelCloser, err := obs.SetupOTel(root, cfg.OTEL.AsOTELConfig())
	if err != nil {
		l.Fatal("otel init", zap.Error(err))
	}
	defer func() { _ = otelCloser.Shutdown(context.Background()) }()

	// db
	db, err := pg.NewDB(root, cfg.DB)
	if err != nil {
		l.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()

	// metrics
	ms := obs.BootstrapMetricsServer(cfg.Server.MetricsAddr, func(ctx context.Context) error {
		hctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		return db.Pool.Ping(hctx)
	}, l)

	// kafka
	cons := kafka.BootstrapConsumer(root, cfg.In.AsConsumerConfig(), l).WithLogger(l)
	defer func() { _ = cons.Close() }()

	prod := kafka.NewProducer(cfg.Out.Brokers, cfg.Out.Topic).WithLogger(l) // todo make bootstrap
	defer func() { _ = prod.Close() }()

	events := kafka.NewCheckEventsKafka(prod)

	// wiring
	outboxRunner, ctrl := wire(cfg, db, events, cons, l)

	// start
	outboxRunner.Start(root)
	errCh := make(chan error, 1)
	go func() { errCh <- ctrl.Run(root) }()

	// loop
	select {
	case <-root.Done():
	case err = <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			l.Error("controller error", zap.Error(err))
		}
	}

	// graceful metrics server shutdown
	shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = ms.Shutdown(shCtx)
	l.Info("bye")
}
