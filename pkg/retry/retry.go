package retry

import (
	"context"
	"github.com/icinga/icingadb/pkg/backoff"
	"github.com/pkg/errors"
	"time"
)

// RetryableFunc is a retryable function.
type RetryableFunc func(context.Context) error

// IsRetryable checks whether a new attempt can be started based on the error passed.
type IsRetryable func(error) bool

// WithBackoff retries the passed function if it fails and the error allows it to retry.
// The specified backoff policy is used to determine how long to sleep between attempts.
// Once the specified timeout (if >0) elapses, WithBackoff gives up.
func WithBackoff(
	ctx context.Context, retryableFunc RetryableFunc, retryable IsRetryable, b backoff.Backoff, timeout time.Duration,
) (err error) {
	if timeout > 0 {
		var cancelCtx context.CancelFunc
		ctx, cancelCtx = context.WithTimeout(ctx, timeout)
		defer cancelCtx()
	}

	for attempt := 0; ; /* true */ attempt++ {
		prevErr := err

		if err = retryableFunc(ctx); err == nil {
			return
		}

		isRetryable := retryable(err)

		if prevErr != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
			err = prevErr
		}

		if !isRetryable {
			err = errors.Wrap(err, "can't retry")

			return
		}

		sleep := b(uint64(attempt))
		select {
		case <-ctx.Done():
			if err == nil {
				err = ctx.Err()
			}
			err = errors.Wrap(err, "can't retry")

			return
		case <-time.After(sleep):
		}
	}
}
