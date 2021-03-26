package retry

import (
	"context"
	"github.com/icinga/icingadb/pkg/backoff"
	"time"
)

// RetryableFunc is a retryable function.
type RetryableFunc func() error

// IsRetryable checks whether a new attempt can be started based on the error passed.
type IsRetryable func(error) bool

// WithBackoff retries the passed function if it fails and the error allows it to retry.
// The specified backoff policy is used to determine how long to sleep between attempts.
func WithBackoff(ctx context.Context, retryableFunc RetryableFunc, retryable IsRetryable, b backoff.Backoff) (err error) {
	for attempt := 0; ; /* true */ attempt++ {
		if err = retryableFunc(); err == nil {
			// No error.
			return
		}

		if !retryable(err) {
			// Not retryable.
			return
		}

		sleep := b(uint64(attempt))
		select {
		case <-ctx.Done():
			// Context canceled. Return last known error.
			if err == nil {
				err = ctx.Err()
			}
			return
		case <-time.After(sleep):
			// Wait for backoff duration and continue.
		}
	}
}
