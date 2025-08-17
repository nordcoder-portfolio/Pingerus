package ping_worker

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	pingcfg "github.com/NordCoder/Pingerus/internal/config/ping-worker"
	"github.com/NordCoder/Pingerus/internal/domain/check"
	"github.com/NordCoder/Pingerus/internal/domain/run"

	kafkax "github.com/NordCoder/Pingerus/internal/repository/kafka"

	pb "github.com/NordCoder/Pingerus/generated/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

type Runner struct {
	log     *zap.Logger
	cons    *kafkax.Consumer
	pub     *kafkax.CheckEventsKafka
	checks  check.Repo
	runs    run.Repo
	httpc   *http.Client
	httpcfg pingcfg.HTTPPing

	mMsgs    prometheus.Counter
	mPings   prometheus.Counter
	mUp      prometheus.Counter
	mDown    prometheus.Counter
	mChanges prometheus.Counter
	mErrors  prometheus.Counter
	mLatency prometheus.Histogram
}

func NewRunner(
	log *zap.Logger,
	cons *kafkax.Consumer,
	pub *kafkax.CheckEventsKafka,
	checks check.Repo,
	runs run.Repo,
	httpc *http.Client,
	httpcfg pingcfg.HTTPPing,
) *Runner {
	return &Runner{
		log:     log,
		cons:    cons,
		pub:     pub,
		checks:  checks,
		runs:    runs,
		httpc:   httpc,
		httpcfg: httpcfg,
		mMsgs: promauto.NewCounter(prometheus.CounterOpts{
			Name: "pinger_messages_consumed_total", Help: "CheckRequest messages consumed",
		}),
		mPings: promauto.NewCounter(prometheus.CounterOpts{
			Name: "pinger_pings_total", Help: "Total pings attempted",
		}),
		mUp: promauto.NewCounter(prometheus.CounterOpts{
			Name: "pinger_up_total", Help: "UP results",
		}),
		mDown: promauto.NewCounter(prometheus.CounterOpts{
			Name: "pinger_down_total", Help: "DOWN results",
		}),
		mChanges: promauto.NewCounter(prometheus.CounterOpts{
			Name: "pinger_status_changes_total", Help: "Status changes emitted",
		}),
		mErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "pinger_errors_total", Help: "Errors",
		}),
		mLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "pinger_latency_seconds",
			Help:    "HTTP ping latency",
			Buckets: prometheus.DefBuckets,
		}),
	}
}

func (r *Runner) Run(ctx context.Context) error {
	handler := kafkax.ProtoHandler(
		func() *pb.CheckRequest { return &pb.CheckRequest{} },
		func(ctx context.Context, _ []byte, msg *pb.CheckRequest) error {
			r.mMsgs.Inc()
			return r.handleCheck(ctx, msg)
		},
	)

	if err := r.cons.Consume(ctx, handler); err != nil && !errors.Is(err, context.Canceled) {
		r.mErrors.Inc()
		r.log.Warn("kafka consume", zap.Error(err))
		return err
	}
	return ctx.Err()
}

func (r *Runner) handleCheck(ctx context.Context, req *pb.CheckRequest) error {
	checkID := int64(req.GetCheckId())
	if checkID <= 0 {
		r.mErrors.Inc()
		r.log.Warn("invalid CheckRequest", zap.Int64("check_id", checkID))
		return nil
	}

	chk, err := r.checks.GetByID(checkID)
	if err != nil {
		r.mErrors.Inc()
		r.log.Warn("get check", zap.Int64("check_id", checkID), zap.Error(err))
		return err
	}

	url := normalizeURL(chk.URL)

	r.mPings.Inc()
	start := time.Now()
	code, status, perr := r.doPing(ctx, url)
	lat := time.Since(start)

	r.mLatency.Observe(lat.Seconds())
	if status {
		r.mUp.Inc()
	} else {
		r.mDown.Inc()
	}

	runRec := &run.Run{
		CheckID:   chk.ID,
		Timestamp: time.Now().UTC(),
		Status:    status,
		Code:      code,
		Latency:   lat.Milliseconds(),
	}
	if err := r.runs.Insert(runRec); err != nil {
		r.mErrors.Inc()
		r.log.Warn("insert run", zap.Int64("check_id", chk.ID), zap.Error(err))
	}

	changed := false
	switch prev := chk.LastStatus; {
	case prev == nil && status:
		changed = true
	case prev != nil && *prev != status:
		changed = true
	}

	if changed {
		newVal := status
		chk.LastStatus = &newVal
		if err := r.checks.Update(chk); err != nil {
			r.mErrors.Inc()
			r.log.Warn("update last_status", zap.Int64("check_id", chk.ID), zap.Error(err))
		}

		r.mChanges.Inc()

		oldStatus := boolFromPtr(runRec.Status != status, chk.LastStatus)

		if chk.LastStatus != nil {
			oldStatus = !newVal
		} else {
			oldStatus = false
		}

		if err := r.pub.PublishStatusChanged(ctx, chk.ID, oldStatus, status); err != nil {
			r.mErrors.Inc()
			r.log.Warn("publish status change", zap.Int64("check_id", chk.ID), zap.Error(err))
		}
	}

	_ = perr
	return nil
}

func (r *Runner) doPing(ctx context.Context, url string) (code int, up bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, false, err
	}
	req.Header.Set("User-Agent", r.httpcfg.UserAgent)

	resp, err := r.httpc.Do(req)
	if err != nil {
		return 0, false, nil
	}
	defer resp.Body.Close()

	code = resp.StatusCode
	up = code >= 200 && code <= 399
	return code, up, nil
}

func normalizeURL(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return t
	}
	if strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") {
		return t
	}
	return "http://" + t
}

func boolFromPtr(_changed bool, p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}
