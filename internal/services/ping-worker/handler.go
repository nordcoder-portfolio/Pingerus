package ping_worker

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/domain/notification"
	"github.com/NordCoder/Pingerus/internal/domain/outbox"
	"github.com/NordCoder/Pingerus/internal/domain/run"
	intoutbox "github.com/NordCoder/Pingerus/internal/outbox"
	"github.com/NordCoder/Pingerus/internal/repository/postgres"
	"github.com/NordCoder/Pingerus/internal/services/ping-worker/repo"
	"strings"
	"time"
)

type Handler struct {
	Checks     repo.CheckRepo
	Runs       repo.RunRepo
	Outbox     outbox.Repository // todo adapter
	Transactor postgres.Transactor
	Events     repo.Events
	Clock      notification.Clock
	HTTP       HTTPPing
}

func (h *Handler) HandleCheck(ctx context.Context, checkID int64) error {
	if checkID <= 0 {
		return nil
	}
	chk, err := h.Checks.GetByID(ctx, checkID)
	if err != nil {
		return fmt.Errorf("get check: %w", err)
	}

	url := normalizeURL(chk.URL)

	start := h.Clock.Now()
	code, status, pingErr := h.HTTP.Do(ctx, url)
	lat := h.Clock.Now().Sub(start)

	_ = h.Runs.Insert(ctx, &run.Run{
		CheckID:   chk.ID,
		Timestamp: h.Clock.Now().UTC(),
		Status:    status,
		Code:      code,
		Latency:   lat.Milliseconds(),
	})

	changed := false
	switch prev := chk.LastStatus; {
	case prev == nil && status:
		changed = true
	case prev != nil && *prev != status:
		changed = true
	}

	if changed {
		old := false
		if chk.LastStatus != nil {
			old = *chk.LastStatus
		}
		newVal := status

		if err := h.Transactor.WithTx(ctx, func(txCtx context.Context) error {
			runRec := &run.Run{
				CheckID:   chk.ID,
				Timestamp: time.Now().UTC(),
				Status:    status,
				Code:      code,
				Latency:   lat.Milliseconds(),
			}
			if err := h.Runs.Insert(txCtx, runRec); err != nil {
				return fmt.Errorf("insert run: %w", err)
			}

			chk.LastStatus = &newVal
			if err := h.Checks.Update(txCtx, chk); err != nil {
				return fmt.Errorf("update check: %w", err)
			}

			payload := intoutbox.StatusChangedPayload{
				CheckID: chk.ID,
				Old:     old,
				New:     newVal,
				At:      time.Now().UTC(),
			}
			b, _ := json.Marshal(payload)
			key := fmt.Sprintf("status:%d:%d", chk.ID, payload.At.UnixNano())

			if err := h.Outbox.Enqueue(txCtx, key, outbox.KindStatusChanged, b); err != nil {
				return fmt.Errorf("outbox enqueue: %w", err)
			}
			return nil
		}); err != nil {
			// todo log
		} else {
			// todo log
		}
	}

	_ = pingErr
	return nil
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
