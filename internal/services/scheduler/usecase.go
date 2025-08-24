package scheduler

import (
	"context"
	"fmt"
	"github.com/NordCoder/Pingerus/internal/services/scheduler/repo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Usecase struct {
	Repo   repo.CheckRepo
	Events repo.Events
}

func NewUC(repo repo.CheckRepo, events repo.Events) *Usecase {
	return &Usecase{Repo: repo, Events: events}
}

func (u *Usecase) Tick(ctx context.Context, limit int) (int, int, int, error) {
	if limit <= 0 {
		limit = 100
	}

	tr := otel.Tracer("scheduler.uc")
	ctxTick, span := tr.Start(ctx, "scheduler.tick",
		trace.WithAttributes(attribute.Int("batch.limit", limit)),
	)
	defer span.End()

	due, err := u.Repo.FetchDue(ctxTick, limit)
	if err != nil {
		span.RecordError(err)
		return 0, 0, 1, fmt.Errorf("fetch due: %w", err)
	}
	if len(due) == 0 {
		span.SetAttributes(attribute.Int("batch.fetched", 0))
		return 0, 0, 0, nil
	}

	span.SetAttributes(attribute.Int("batch.fetched", len(due)))

	sent, errs := 0, 0
	for _, c := range due {
		_, sp := tr.Start(ctxTick, "scheduler.publish",
			trace.WithAttributes(
				attribute.Int64("check.id", c.ID),
				attribute.String("check.url", c.URL),
			),
		)
		pubErr := u.Events.PublishCheckRequested(ctxTick, c.ID)
		if pubErr != nil {
			errs++
			sp.RecordError(pubErr)
			sp.SetAttributes(attribute.String("publish.status", "error"))
			sp.End()
			continue
		}
		sent++
		sp.SetAttributes(attribute.String("publish.status", "ok"))
		sp.End()
	}

	span.SetAttributes(
		attribute.Int("batch.sent", sent),
		attribute.Int("batch.errors", errs),
	)
	return len(due), sent, errs, nil
}
