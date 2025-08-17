package notifier

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NordCoder/Pingerus/internal/domain/check"
	"github.com/NordCoder/Pingerus/internal/domain/notification"
	"github.com/NordCoder/Pingerus/internal/domain/user"

	kafkax "github.com/NordCoder/Pingerus/internal/repository/kafka"

	pb "github.com/NordCoder/Pingerus/generated/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

type Runner struct {
	log    *zap.Logger
	cons   *kafkax.Consumer
	mail   *Mailer
	checks check.Repo
	users  user.Repo
	notifs notification.Repo

	mConsumed prometheus.Counter
	mSent     prometheus.Counter
	mErrors   prometheus.Counter
}

func NewRunner(
	log *zap.Logger,
	cons *kafkax.Consumer,
	mail *Mailer,
	checks check.Repo,
	users user.Repo,
	notifs notification.Repo,
) *Runner {
	return &Runner{
		log:    log,
		cons:   cons,
		mail:   mail,
		checks: checks,
		users:  users,
		notifs: notifs,
		mConsumed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "email_notifier_messages_consumed_total",
			Help: "StatusChange events consumed",
		}),
		mSent: promauto.NewCounter(prometheus.CounterOpts{
			Name: "email_notifier_emails_sent_total",
			Help: "Emails sent",
		}),
		mErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "email_notifier_errors_total",
			Help: "Errors",
		}),
	}
}

func (r *Runner) Run(ctx context.Context) error {
	handler := kafkax.ProtoHandler(
		func() *pb.StatusChange { return &pb.StatusChange{} },
		func(ctx context.Context, _ []byte, ev *pb.StatusChange) error {
			r.mConsumed.Inc()
			if ev.GetCheckId() <= 0 {
				r.log.Warn("invalid StatusChange: bad check_id", zap.Int32("check_id", ev.GetCheckId()))
				return nil
			}
			return r.handleStatusChange(ctx, ev)
		},
	)

	if err := r.cons.Consume(ctx, handler); err != nil && !errors.Is(err, context.Canceled) {
		r.mErrors.Inc()
		r.log.Warn("kafka consume", zap.Error(err))
		return err
	}
	return ctx.Err()
}

func (r *Runner) handleStatusChange(ctx context.Context, ev *pb.StatusChange) error {
	checkID := int64(ev.GetCheckId())

	chk, err := r.checks.GetByID(checkID)
	if err != nil {
		r.mErrors.Inc()
		return fmt.Errorf("get check: %w", err)
	}

	u, err := r.users.GetByID(chk.UserID)
	if err != nil {
		r.mErrors.Inc()
		return fmt.Errorf("get user: %w", err)
	}

	subject := fmt.Sprintf("Site status changed: %t → %t", ev.GetOldStatus(), ev.GetNewStatus())
	body := fmt.Sprintf(
		"Hello!\n\nYour check (%s) changed status: %t → %t at %s.\n\n— Pingerus",
		chk.URL, ev.GetOldStatus(), ev.GetNewStatus(), ev.GetTs().AsTime().Format(time.RFC3339),
	)

	if err := r.mail.Send(ctx, u.Email, subject, body); err != nil {
		r.mErrors.Inc()
		return fmt.Errorf("send email: %w", err)
	}
	r.mSent.Inc()

	if r.notifs != nil {
		n := &notification.Notification{
			CheckID: checkID,
			UserID:  chk.UserID,
			Type:    "email",
			SentAt:  time.Now().UTC(),
			Payload: body,
		}
		_ = r.notifs.Create(n) // todo для MVP — без эскалации ошибки
	}

	return nil
}
