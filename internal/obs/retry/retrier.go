package retry

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel/trace"
)

type Backoff interface {
	Next(attempt int) time.Duration
}

type ExpoJitter struct {
	Base   time.Duration
	Max    time.Duration
	Jitter float64
}

func (b ExpoJitter) Next(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	d := float64(b.Base) * math.Pow(2, float64(attempt))
	if b.Max > 0 && time.Duration(d) > b.Max {
		d = float64(b.Max)
	}
	if b.Jitter > 0 {
		j := 1 + (rand.Float64()*2-1)*b.Jitter
		d *= j
	}
	return time.Duration(d)
}

type Policy struct {
	Name      string
	Attempts  int
	Backoff   Backoff
	Retryable func(error) bool
	OnAttempt func(attempt int, err error)
	OnExhaust func(lastErr error)
}

var (
	retryAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "retry_attempts_total",
		Help: "Total retry attempts (including final).",
	}, []string{"name"})
	retryExhausted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "retry_exhausted_total",
		Help: "Operations that exhausted all retries.",
	}, []string{"name"})
	retryLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "retry_duration_seconds",
		Help:    "Total time spent inside retry.Do (success or fail).",
		Buckets: prometheus.DefBuckets,
	}, []string{"name"})
)

func Do(ctx context.Context, fn func() error, p Policy) error {
	start := time.Now()
	name := p.Name
	if name == "" {
		name = "default"
	}

	attempts := p.Attempts
	if attempts <= 0 {
		attempts = 1
	}

	isRetryable := p.Retryable
	if isRetryable == nil {
		isRetryable = func(err error) bool { return err != nil }
	}

	var err error
	span := trace.SpanFromContext(ctx)

	for i := 0; i < attempts; i++ {
		err = fn()
		retryAttempts.WithLabelValues(name).Inc()
		if err == nil {
			retryLatency.WithLabelValues(name).Observe(time.Since(start).Seconds())
			return nil
		}
		if p.OnAttempt != nil {
			p.OnAttempt(i, err)
		}
		if span.IsRecording() {
			span.AddEvent("retry.attempt", trace.WithAttributes())
		}
		if !isRetryable(err) || i == attempts-1 {
			retryExhausted.WithLabelValues(name).Inc()
			if p.OnExhaust != nil {
				p.OnExhaust(err)
			}
			retryLatency.WithLabelValues(name).Observe(time.Since(start).Seconds())
			return err
		}
		wait := p.Backoff.Next(i)
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			retryLatency.WithLabelValues(name).Observe(time.Since(start).Seconds())
			return ctx.Err()
		case <-t.C:
		}
	}
	retryLatency.WithLabelValues(name).Observe(time.Since(start).Seconds())
	return err
}
