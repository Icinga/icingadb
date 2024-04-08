package retry

import (
	"context"
	"database/sql/driver"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icingadb/pkg/backoff"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"net"
	"syscall"
	"time"
)

// DefaultTimeout is our opinionated default timeout for retrying database and Redis operations.
const DefaultTimeout = 5 * time.Minute

// RetryableFunc is a retryable function.
type RetryableFunc func(context.Context) error

// IsRetryable checks whether a new attempt can be started based on the error passed.
type IsRetryable func(error) bool

// Settings aggregates optional settings for WithBackoff.
type Settings struct {
	// If >0, Timeout lets WithBackoff stop retrying once elapsed,
	// but allows the RetryableFunc its execution time and **doesn't abort** it if it exceeds Timeout.
	// This means that WithBackoff may not stop exactly after Timeout expires,
	// or may not retry at all if the first execution of RetryableFunc already takes longer than Timeout.
	Timeout time.Duration
	// OnRetryableError is called if a retryable error occurs.
	OnRetryableError func(elapsed time.Duration, attempt uint64, err, lastErr error)
	// OnSuccess is called once the operation succeeds.
	OnSuccess func(elapsed time.Duration, attempt uint64, lastErr error)
}

// WithBackoff retries the passed function if it fails and the error allows it to retry.
// The specified backoff policy is used to determine how long to sleep between attempts.
func WithBackoff(
	ctx context.Context, retryableFunc RetryableFunc, retryable IsRetryable, b backoff.Backoff, settings Settings,
) (err error) {
	// Channel for retry deadline, which is set to the channel of NewTimer() if a timeout is configured,
	// otherwise nil, so that it blocks forever if there is no timeout.
	var timeout <-chan time.Time

	if settings.Timeout > 0 {
		t := time.NewTimer(settings.Timeout)
		defer t.Stop()
		timeout = t.C
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

		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			if prevErr != nil {
				err = errors.Wrap(err, prevErr.Error())
			}

			return
		}

		if !retryable(err) {
			err = errors.Wrap(err, "can't retry")

			return
		}

		if settings.OnRetryableError != nil {
			settings.OnRetryableError(time.Since(start), attempt, err, prevErr)
		}

		select {
		case <-time.After(b(attempt)):
		case <-timeout:
			err = errors.Wrap(err, "retry deadline exceeded")

			return
		case <-ctx.Done():
			err = errors.Wrap(ctx.Err(), err.Error())

			return
		}
	}
}

// ResetTimeout changes the possibly expired timer t to expire after duration d.
//
// If the timer has already expired and nothing has been received from its channel,
// it is automatically drained as if the timer had never expired.
func ResetTimeout(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		<-t.C
	}

	t.Reset(d)
}

// Retryable returns true for common errors that are considered retryable,
// i.e. temporary, timeout, DNS, connection refused and reset, host down and unreachable and
// network down and unreachable errors. In addition, any database error is considered retryable.
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
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ENOENT) {
		// syscall errors provide Temporary() and Timeout(),
		// which do not include ECONNREFUSED or ENOENT, so we check these ourselves.
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
	if errors.Is(err, syscall.EPIPE) {
		return true
	}

	if errors.Is(err, driver.ErrBadConn) {
		return true
	}
	if errors.Is(err, mysql.ErrInvalidConn) {
		return true
	}

	var mye *mysql.MySQLError
	var pqe *pq.Error
	if errors.As(err, &mye) || errors.As(err, &pqe) {
		return true
	}

	return false
}
