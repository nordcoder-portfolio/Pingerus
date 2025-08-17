package notifier

import (
	"context"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/domain/notification"
	"github.com/NordCoder/Pingerus/internal/services/email-notifier/repo"
	"time"
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
}

func (h *Handler) HandleStatusChange(ctx context.Context, ev StatusChange) error {
	chk, err := h.Checks.GetByID(ctx, ev.CheckID)
	if err != nil {
		return fmt.Errorf("get check: %w", err)
	}

	u, err := h.Users.GetByID(ctx, chk.UserID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	subject := fmt.Sprintf("Site status changed: %t → %t", ev.OldStatus, ev.NewStatus)
	body := fmt.Sprintf(
		"Hello!\n\nYour check (%s) changed status: %t → %t at %s.\n\n— Pingerus",
		chk.URL, ev.OldStatus, ev.NewStatus, ev.At.UTC().Format(time.RFC3339),
	)

	if err := h.Out.Send(ctx, u.Email, subject, body); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	_ = h.Store.Create(ctx, &notification.Notification{
		CheckID: chk.ID,
		UserID:  u.ID,
		Type:    "email",
		SentAt:  h.Clock.Now().UTC(),
		Payload: body,
	})

	return nil
}
