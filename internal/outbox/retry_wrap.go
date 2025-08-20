package outbox

import (
	"context"

	"github.com/NordCoder/Pingerus/internal/domain/outbox"
	"github.com/NordCoder/Pingerus/internal/obs/retry"
)

func WrapKindHandler(h outbox.KindHandler, p retry.Policy) outbox.KindHandler {
	return func(ctx context.Context, data []byte) error {
		return retry.Do(ctx, func() error { return h(ctx, data) }, p)
	}
}
