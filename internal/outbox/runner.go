package outbox

import (
	"context"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"strconv"
	"sync"
	"time"

	"github.com/NordCoder/Pingerus/internal/domain/outbox"
	"github.com/NordCoder/Pingerus/internal/obs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type Runner struct {
	log      *zap.Logger
	repo     outbox.Repository
	dispatch outbox.GlobalHandler

	workers       int
	batchSize     int
	waitTime      time.Duration
	inProgressTTL time.Duration

	mPicked    prometheus.Counter
	mOk        prometheus.Counter
	mErr       prometheus.Counter
	mTickDur   prometheus.Histogram
	mBatchSize prometheus.Gauge
}

func NewOutboxRunner(
	log *zap.Logger,
	repo outbox.Repository,
	dispatch outbox.GlobalHandler,
	workers int,
	batchSize int,
	waitTime time.Duration,
	inProgressTTL time.Duration,
) *Runner {
	return &Runner{
		log: log, repo: repo, dispatch: dispatch,
		workers: workers, batchSize: batchSize, waitTime: waitTime, inProgressTTL: inProgressTTL,
		mPicked: promauto.NewCounter(prometheus.CounterOpts{
			Name: "outbox_picked_total", Help: "Messages picked into processing.",
		}),
		mOk: promauto.NewCounter(prometheus.CounterOpts{
			Name: "outbox_processed_ok_total", Help: "Messages processed successfully.",
		}),
		mErr: promauto.NewCounter(prometheus.CounterOpts{
			Name: "outbox_processed_err_total", Help: "Handler errors.",
		}),
		mTickDur: promauto.NewHistogram(prometheus.HistogramOpts{
			Name: "outbox_tick_duration_seconds", Help: "Tick duration.",
			Buckets: prometheus.DefBuckets,
		}),
		mBatchSize: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "outbox_last_batch_size", Help: "Size of last picked batch.",
		}),
	}
}

func (r *Runner) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < r.workers; i++ {
		wg.Add(1)
		go r.worker(ctx, &wg)
	}
}

func (r *Runner) worker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	r.log.Info("outbox worker started", zap.String("wait_ms", strconv.FormatInt(r.waitTime.Milliseconds(), 10)))

	ticker := time.NewTicker(r.waitTime)
	defer ticker.Stop()

	tr := otel.Tracer("outbox.runner")
	prop := otel.GetTextMapPropagator()

	for {
		select {
		case <-ctx.Done():
			r.log.Info("outbox worker stop")
			return

		case <-ticker.C:
			t0 := time.Now()

			ctxSpan, span := tr.Start(ctx, "outbox.tick")
			span.SetAttributes(
				attribute.Int("batch.limit", r.batchSize),
				attribute.String("in_progress_ttl", r.inProgressTTL.String()),
			)

			messages, err := r.repo.PickBatch(ctxSpan, r.batchSize, r.inProgressTTL)
			if err != nil {
				span.RecordError(err)
				r.mErr.Inc()
				obs.WithTrace(ctxSpan, r.log).Error("outbox pick error", zap.Error(err))
				span.End()
				continue
			}
			r.mPicked.Add(float64(len(messages)))
			r.mBatchSize.Set(float64(len(messages)))

			okKeys := make([]string, 0, len(messages))

			for _, m := range messages {
				parent := prop.Extract(context.Background(), propagation.MapCarrier{
					"traceparent": m.Traceparent,
					"tracestate":  m.Tracestate,
					"baggage":     m.Baggage,
				})

				msgCtx, msgSpan := tr.Start(parent, "outbox.dispatch",
					trace.WithAttributes(
						attribute.String("outbox.key", m.IdempotencyKey),
						attribute.Int("outbox.kind", int(m.Kind)),
					),
				)

				handler, herr := r.dispatch(m.Kind)
				if herr != nil {
					msgSpan.RecordError(herr)
					r.mErr.Inc()
					obs.WithTrace(msgCtx, r.log).Error("no handler for kind",
						zap.Int("kind", int(m.Kind)), zap.Error(herr))
					msgSpan.End()
					continue
				}

				if err := handler(msgCtx, m.Data); err != nil {
					msgSpan.RecordError(err)
					r.mErr.Inc()
					obs.WithTrace(msgCtx, r.log).Error("handler error",
						zap.Int("kind", int(m.Kind)), zap.Error(err))
					msgSpan.End()
					continue
				}

				msgSpan.End()
				okKeys = append(okKeys, m.IdempotencyKey)
				r.mOk.Inc()
			}

			if err := r.repo.MarkSuccess(ctxSpan, okKeys); err != nil {
				span.RecordError(err)
				r.mErr.Inc()
				obs.WithTrace(ctxSpan, r.log).Error("mark success error", zap.Error(err))
			}

			span.End()
			r.mTickDur.Observe(time.Since(t0).Seconds())
		}
	}
}
