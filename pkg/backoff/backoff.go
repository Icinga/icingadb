package backoff

import (
	"math/rand"
	"time"
)

// Backoff returns the backoff duration for a specific retry attempt.
type Backoff func(uint64) time.Duration

// NewExponentialWithJitter returns a backoff implementation that
// exponentially increases the backoff duration for each retry from min,
// never exceeding max. Some randomization is added to the backoff duration.
// It panics if min >= max.
func NewExponentialWithJitter(min, max time.Duration) Backoff {
	if min <= 0 {
		min = time.Millisecond * 100
	}
	if max <= 0 {
		max = time.Second * 10
	}
	if min >= max {
		panic("max must be larger than min")
	}

	return func(attempt uint64) time.Duration {
		e := min << attempt
		if e <= 0 || e > max {
			e = max
		}

		return time.Duration(jitter(int64(e)))
	}
}

// jitter returns a random integer distributed in the range [n/2..n).
func jitter(n int64) int64 {
	if n == 0 {
		return 0
	}

	return n/2 + rand.Int63n(n/2)
}
