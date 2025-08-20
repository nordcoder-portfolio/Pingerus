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

type Handler struct {
	Checks repo.CheckReader
	Users  repo.UserReader
	Store  repo.NotificationRepo
	Out    notification.EmailSender
	Clock  notification.Clock

	// необязательно, но удобно для детальных логов
	Log *zap.Logger
}

func (h *Handler) HandleStatusChange(ctx context.Context, ev StatusChange) error {
	log := h.Log
	if log == nil {
		log = zap.NewNop()
	}
	log = log.With(zap.String("component", "email-notifier.handler"), zap.Int64("check_id", ev.CheckID))

	start := time.Now()
	chk, err := h.Checks.GetByID(ctx, ev.CheckID)
	if err != nil {
		log.Error("get check failed", zap.Error(err))
		return fmt.Errorf("get check: %w", err)
	}
	log.Debug("check loaded",
		zap.Int64("user_id", chk.UserID),
		zap.String("url", chk.URL),
	)

	u, err := h.Users.GetByID(ctx, chk.UserID)
	if err != nil {
		log.Error("get user failed", zap.Int64("user_id", chk.UserID), zap.Error(err))
		return fmt.Errorf("get user: %w", err)
	}
	log.Debug("user loaded", zap.Int64("user_id", u.ID), zap.String("email", u.Email))

	subject := fmt.Sprintf("Site status changed: %t → %t", ev.OldStatus, ev.NewStatus)
	body := fmt.Sprintf(
		"Hello!\n\nYour check (%s) changed status: %t → %t at %s.\n\n— Pingerus",
		chk.URL, ev.OldStatus, ev.NewStatus, ev.At.UTC().Format(time.RFC3339),
	)

	sendStart := time.Now()
	if err := h.Out.Send(ctx, u.Email, subject, body); err != nil {
		log.Error("send email failed",
			zap.String("to", u.Email),
			zap.String("subject", subject),
			zap.Duration("elapsed", time.Since(sendStart)),
			zap.Error(err),
		)
		return fmt.Errorf("send email: %w", err)
	}
	log.Info("email sent",
		zap.String("to", u.Email),
		zap.String("subject", subject),
		zap.Duration("elapsed", time.Since(sendStart)),
	)

	// best-effort запись события (логируем только ошибку, но не фэйлим общий пайп)
	if err := h.Store.Create(ctx, &notification.Notification{
		CheckID: chk.ID,
		UserID:  u.ID,
		Type:    "email",
		SentAt:  h.Clock.Now().UTC(),
		Payload: body,
	}); err != nil {
		log.Warn("store notification failed",
			zap.Int64("user_id", u.ID),
			zap.Int64("check_id", chk.ID),
			zap.Error(err),
		)
	} else {
		log.Debug("notification stored",
			zap.Int64("user_id", u.ID),
			zap.Int64("check_id", chk.ID),
		)
	}

	log.Info("status-change processed",
		zap.Duration("elapsed", time.Since(start)),
	)
	return nil
}
