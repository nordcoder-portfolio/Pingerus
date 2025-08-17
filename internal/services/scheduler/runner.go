package scheduler

import (
	"context"
	scheduler2 "github.com/NordCoder/Pingerus/internal/config/scheduler"
	"github.com/NordCoder/Pingerus/internal/domain/check"
	"github.com/NordCoder/Pingerus/internal/repository/kafka"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

type Runner struct {
	log  *zap.Logger
	repo check.Repo
	pub  *kafka.CheckEventsKafka
	cfg  scheduler2.SchedCfg

	mFetched prometheus.Counter
	mSent    prometheus.Counter
	mErr     prometheus.Counter
	mLoopDur prometheus.Histogram
}

func NewRunner(log *zap.Logger, repo check.Repo, pub *kafka.CheckEventsKafka, cfg scheduler2.SchedCfg) *Runner {
	return &Runner{
		log:  log,
		repo: repo,
		pub:  pub,
		cfg:  cfg,
		mFetched: promauto.NewCounter(prometheus.CounterOpts{
			Name: "scheduler_checks_fetched_total", Help: "Due checks fetched from DB",
		}),
		mSent: promauto.NewCounter(prometheus.CounterOpts{
			Name: "scheduler_messages_sent_total", Help: "CheckRequest published to Kafka",
		}),
		mErr: promauto.NewCounter(prometheus.CounterOpts{
			Name: "scheduler_errors_total", Help: "Errors in scheduler loop",
		}),
		mLoopDur: promauto.NewHistogram(prometheus.HistogramOpts{
			Name: "scheduler_loop_duration_seconds", Help: "Scheduler tick duration",
			Buckets: prometheus.DefBuckets,
		}),
	}
}

func (r *Runner) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.cfg.Tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			start := time.Now()
			r.tick(ctx)
			r.mLoopDur.Observe(time.Since(start).Seconds())
		}
	}
}

func (r *Runner) tick(ctx context.Context) {
	limit := r.cfg.BatchLimit
	if limit <= 0 {
		limit = 100
	}

	due, err := r.repo.FetchDue(limit)
	if err != nil {
		r.mErr.Inc()
		r.log.Warn("fetch due failed", zap.Error(err))
		return
	}
	if len(due) == 0 {
		return
	}
	r.mFetched.Add(float64(len(due)))

	for _, c := range due {
		if err := r.pub.PublishCheckRequested(ctx, c.ID); err != nil {
			r.mErr.Inc()
			r.log.Warn("publish failed", zap.Int64("check_id", c.ID), zap.Error(err))
			continue
		}
		r.mSent.Inc()
	}
	r.log.Debug("scheduled batch", zap.Int("count", len(due)))
}
