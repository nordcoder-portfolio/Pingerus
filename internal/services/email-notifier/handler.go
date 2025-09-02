package notifier

import (
	"context"
	"fmt"
	"time"

	"github.com/NordCoder/Pingerus/internal/domain/notification"
	"github.com/NordCoder/Pingerus/internal/services/email-notifier/repo"
	"go.uber.org/zap"
)

type StatusChange struct {
	CheckID   int64
	OldStatus bool
	NewStatus bool
	At        time.Time
}

const notificationTypeEmail = "email"

type Handler struct {
	Checks repo.CheckReader
	Users  repo.UserReader
	Store  repo.NotificationRepo
	Out    notification.EmailSender
	Clock  notification.Clock
	Log    *zap.Logger
}

func (h *Handler) logger() *zap.Logger {
	if h.Log != nil {
		return h.Log
	}
	return zap.NewNop()
}

func (h *Handler) HandleStatusChange(ctx context.Context, ev StatusChange) error {
	log := h.logger().With(
		zap.String("component", "email-notifier.handler"),
		zap.Int64("check_id", ev.CheckID),
		zap.Bool("old_status", ev.OldStatus),
		zap.Bool("new_status", ev.NewStatus),
		zap.Time("event_at", ev.At.UTC()),
	)

	start := h.Clock.Now()
	defer func() { log.Info("status-change processed", zap.Duration("elapsed", h.Clock.Now().Sub(start))) }()

	chk, err := h.Checks.GetByID(ctx, ev.CheckID)
	if err != nil {
		log.Error("get check failed", zap.Error(err))
		return fmt.Errorf("get check: %w", err)
	}

	log = log.With(zap.Int64("user_id", chk.UserID), zap.String("url", chk.URL))
	log.Debug("check loaded")

	u, err := h.Users.GetByID(ctx, chk.UserID)
	if err != nil {
		log.Error("get user failed", zap.Error(err))
		return fmt.Errorf("get user: %w", err)
	}
	if u.Email == "" {
		err := fmt.Errorf("user has no email")
		log.Error("missing recipient email", zap.Error(err))
		return err
	}
	log.Debug("user loaded", zap.String("email", u.Email))

	subject, body := buildEmail(chk.URL, ev, h.Clock)

	sendStart := h.Clock.Now()
	if err := h.Out.Send(ctx, u.Email, subject, body); err != nil {
		log.Error("send email failed",
			zap.String("to", u.Email),
			zap.String("subject", subject),
			zap.Duration("elapsed", h.Clock.Now().Sub(sendStart)),
			zap.Error(err),
		)
		return fmt.Errorf("send email: %w", err)
	}
	log.Info("email sent",
		zap.String("to", u.Email),
		zap.String("subject", subject),
		zap.Duration("elapsed", h.Clock.Now().Sub(sendStart)),
	)

	if err := h.Store.Create(ctx, &notification.Notification{
		CheckID: chk.ID,
		UserID:  u.ID,
		Type:    notificationTypeEmail,
		SentAt:  h.Clock.Now().UTC(),
		Payload: body,
	}); err != nil {
		log.Warn("store notification failed", zap.Error(err))
	} else {
		log.Debug("notification stored")
	}

	return nil
}

func buildEmail(url string, ev StatusChange, clk notification.Clock) (subject, body string) {
	subject = fmt.Sprintf("Site status changed: %t → %t", ev.OldStatus, ev.NewStatus)
	body = fmt.Sprintf(
		"Hello!\n\nYour check (%s) changed status: %t → %t at %s.\n\n— Pingerus",
		url, ev.OldStatus, ev.NewStatus, clk.Now().UTC().Format(time.RFC3339),
	)
	return
}
