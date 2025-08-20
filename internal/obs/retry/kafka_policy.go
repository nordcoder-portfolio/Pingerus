package retry

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
)

func DefaultKafkaPolicy(log *zap.Logger) Policy {
	return Policy{
		Attempts: 6,
		Backoff:  ExpoJitter{Base: 200 * time.Millisecond, Max: 30 * time.Second, Jitter: 0.2},
		Retryable: func(err error) bool {
			return err != nil
		},
		OnAttempt: func(i int, err error) {
			if log != nil {
				log.Warn("outbox retry", zap.Int("attempt", i+1), zap.Error(err))
			}
		},
		OnExhaust: func(err error) {
			if log != nil && !errors.Is(err, context.Canceled) {
				log.Error("outbox retries exhausted", zap.Error(err))
			}
		},
	}
}
