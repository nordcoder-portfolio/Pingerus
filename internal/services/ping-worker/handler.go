package ping_worker

import (
	"context"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/domain/notification"
	"github.com/NordCoder/Pingerus/internal/domain/run"
	"github.com/NordCoder/Pingerus/internal/services/ping-worker/repo"
	"strings"
)

type Handler struct {
	Checks repo.CheckRepo
	Runs   repo.RunRepo
	Events repo.Events
	Clock  notification.Clock
	HTTP   HTTPPing
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
			old = !status
		}
		newVal := status
		chk.LastStatus = &newVal
		if err := h.Checks.Update(ctx, chk); err != nil {
		}
		_ = h.Events.PublishStatusChanged(ctx, chk.ID, old, status)
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
