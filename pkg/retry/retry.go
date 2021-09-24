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

// Settings aggregates optional settings for WithBackoff.
type Settings struct {
	// Timeout lets WithBackoff give up once elapsed (if >0).
	Timeout time.Duration
	// OnError is called if an error occurs.
	OnError func(elapsed time.Duration, attempt uint64, err, lastErr error)
	// OnSuccess is called once the operation succeeds.
	OnSuccess func(elapsed time.Duration, attempt uint64, lastErr error)
}

// WithBackoff retries the passed function if it fails and the error allows it to retry.
// The specified backoff policy is used to determine how long to sleep between attempts.
func WithBackoff(
	ctx context.Context, retryableFunc RetryableFunc, retryable IsRetryable, b backoff.Backoff, settings Settings,
) (err error) {
	if settings.Timeout > 0 {
		var cancelCtx context.CancelFunc
		ctx, cancelCtx = context.WithTimeout(ctx, settings.Timeout)
		defer cancelCtx()
	}

	start := time.Now()
	for attempt := uint64(0); ; /* true */ attempt++ {
		prevErr := err

		if err = retryableFunc(ctx); err == nil {
			if settings.OnSuccess != nil {
				settings.OnSuccess(time.Since(start), attempt, prevErr)
			}

			return
		}

		if settings.OnError != nil {
			settings.OnError(time.Since(start), attempt, err, prevErr)
		}

		isRetryable := retryable(err)

		if prevErr != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
			err = prevErr
		}

		if !isRetryable {
			err = errors.Wrap(err, "can't retry")

			return
		}

		sleep := b(attempt)
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
