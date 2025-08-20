package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NordCoder/Pingerus/internal/domain/outbox"
	"github.com/NordCoder/Pingerus/internal/obs/retry"
	kafkax "github.com/NordCoder/Pingerus/internal/repository/kafka"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
)

type StatusChangedPayload struct {
	CheckID int64     `json:"check_id"`
	Old     bool      `json:"old"`
	New     bool      `json:"new"`
	At      time.Time `json:"at"`
}

var (
	outboxHandlerLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "outbox_handler_latency_seconds",
		Help:    "Latency of outbox handlers (publish, http, etc.)",
		Buckets: prometheus.DefBuckets,
	}, []string{"kind"})
	outboxHandlerErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "outbox_handler_errors_total",
		Help: "Errors in outbox handlers (after retries).",
	}, []string{"kind"})
)

func instrument(kind string, h outbox.KindHandler, pol retry.Policy) outbox.KindHandler {
	tr := otel.Tracer("outbox.handler")
	if pol.Name == "" {
		pol.Name = "outbox_" + kind
	}
	return func(ctx context.Context, data []byte) error {
		ctx, span := tr.Start(ctx, "outbox.handle")
		defer span.End()

		start := time.Now()
		err := retry.Do(ctx, func() error { return h(ctx, data) }, pol)
		outboxHandlerLatency.WithLabelValues(kind).Observe(time.Since(start).Seconds())
		if err != nil {
			span.RecordError(err)
			outboxHandlerErrors.WithLabelValues(kind).Inc()
		}
		return err
	}
}

func MakeGlobalOutboxHandler(pub *kafkax.CheckEventsKafka, pol retry.Policy) outbox.GlobalHandler {
	return func(kind outbox.Kind) (outbox.KindHandler, error) {
		switch kind {
		case outbox.KindStatusChanged:
			base := func(ctx context.Context, data []byte) error {
				var p StatusChangedPayload
				if err := json.Unmarshal(data, &p); err != nil {
					return fmt.Errorf("unmarshal status-changed payload: %w", err)
				}
				return pub.PublishStatusChanged(ctx, p.CheckID, p.Old, p.New)
			}
			return instrument("status_changed", base, pol), nil
		default:
			return nil, fmt.Errorf("unsupported outbox kind: %d", kind)
		}
	}
}
