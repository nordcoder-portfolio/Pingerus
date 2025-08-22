package scheduler

import (
	"context"
	config "github.com/NordCoder/Pingerus/internal/config/scheduler"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

type Runner struct {
	Log *zap.Logger
	UC  *Usecase
	Cfg *config.SchedCfg

	mFetched prometheus.Counter
	mSent    prometheus.Counter
	mErr     prometheus.Counter
	mLoopDur prometheus.Histogram
}

func New(log *zap.Logger, uc *Usecase, cfg *config.SchedCfg) *Runner {
	return &Runner{
		Log: log,
		UC:  uc,
		Cfg: cfg,
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

func (r *Runner) tick(ctx context.Context) {
	start := time.Now()
	fetched, sent, errs, err := r.UC.Tick(ctx, r.Cfg.BatchLimit)
	if err != nil {
		r.mErr.Inc()
		r.Log.Warn("tick error", zap.Error(err))
	}
	if fetched > 0 {
		r.mFetched.Add(float64(fetched))
		r.mSent.Add(float64(sent))
		if errs > 0 {
			r.mErr.Add(float64(errs))
		}
		r.Log.Debug("scheduled batch", zap.Int("fetched", fetched), zap.Int("sent", sent), zap.Int("errors", errs))

	}
	r.mLoopDur.Observe(time.Since(start).Seconds())
}

func (r *Runner) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.Cfg.Tick)
	defer ticker.Stop()

	r.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}
