package retry

import (
	"context"
	"github.com/icinga/icingadb/pkg/backoff"
	"github.com/pkg/errors"
	"net"
	"syscall"
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
	parentCtx := ctx

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
			if outerErr := parentCtx.Err(); outerErr != nil {
				err = errors.Wrap(outerErr, "outer context canceled")
			} else {
				if err == nil {
					err = ctx.Err()
				}
				err = errors.Wrap(err, "can't retry")
			}

			return
		case <-time.After(sleep):
		}
	}
}

// Retryable returns true for common errors that are considered retryable,
// i.e. temporary, timeout, DNS, connection refused and reset, host down and unreachable and
// network down and unreachable errors.
func Retryable(err error) bool {
	var temporary interface {
		Temporary() bool
	}
	if errors.As(err, &temporary) && temporary.Temporary() {
		return true
	}

	var timeout interface {
		Timeout() bool
	}
	if errors.As(err, &timeout) && timeout.Timeout() {
		return true
	}

	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		return true
	}

	var opError *net.OpError
	if errors.As(err, &opError) {
		// OpError provides Temporary() and Timeout(), but not Unwrap(),
		// so we have to extract the underlying error ourselves to also check for ECONNREFUSED,
		// which is not considered temporary or timed out by Go.
		err = opError.Err
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		// syscall errors provide Temporary() and Timeout(),
		// which do not include ECONNREFUSED, so we check this ourselves.
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) {
		// ECONNRESET is treated as a temporary error by Go only if it comes from calling accept.
		return true
	}
	if errors.Is(err, syscall.EHOSTDOWN) || errors.Is(err, syscall.EHOSTUNREACH) {
		return true
	}
	if errors.Is(err, syscall.ENETDOWN) || errors.Is(err, syscall.ENETUNREACH) {
		return true
	}

	return false
}
