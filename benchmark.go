package icingadb_utils

import (
	"fmt"
	"time"
)

// Benchmark is a stopwatch for benchmarking.
type Benchmark struct {
	start time.Time
	end   time.Time
	diff  time.Duration
}

// MarshalText implements an interface from TextMarshaler
func (b *Benchmark) MarshalText() (text []byte, err error) {
	return []byte(b.String()), nil
}

// Stop stops the stopwatch.
func (b *Benchmark) Stop() {
	b.end = time.Now()
	b.diff = b.end.Sub(b.start)
}

// String renders the measured time human-readably.
func (b *Benchmark) String() string {
	var unitDiff time.Duration
	var unit string

	if b.diff < time.Second {
		if b.diff < time.Millisecond {
			unitDiff = time.Nanosecond
			unit = "ns"
		} else {
			unitDiff = time.Millisecond
			unit = "ms"
		}
	} else {
		unitDiff = time.Second
		unit = "s"
	}

	return fmt.Sprintf("%f%s", float64(b.diff)/float64(unitDiff), unit)
}

// Seconds returns the measured time in seconds.
func (b *Benchmark) Seconds() float64 {
	return float64(b.diff) / float64(time.Second)
}

// NewBenchmark constructs and starts a Benchmark.
func NewBenchmark() *Benchmark {
	now := time.Now()

	return &Benchmark{
		start: now,
		end:   now,
		diff:  0,
	}
}
