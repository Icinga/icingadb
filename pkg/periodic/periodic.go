package periodic

import (
	"context"
	"sync"
	"time"
)

// Option configures Start.
type Option interface {
	apply(*periodic)
}

// Stopper implements the Stop method,
// which stops a periodic task from Start().
type Stopper interface {
	Stop() // Stops a periodic task.
}

// Tick is the value for periodic task callbacks that
// contains the time of the tick and
// the time elapsed since the start of the periodic task.
type Tick struct {
	Elapsed time.Duration
	Time    time.Time
}

// Immediate starts the periodic task immediately instead of after the first tick.
func Immediate() Option {
	return optionFunc(func(p *periodic) {
		p.immediate = true
	})
}

// OnStop configures a callback that is executed when a periodic task is stopped or canceled.
func OnStop(f func(Tick)) Option {
	return optionFunc(func(p *periodic) {
		p.onStop = f
	})
}

// Start starts a periodic task with a ticker at the specified interval,
// which executes the given callback after each tick.
// Pending tasks do not overlap, but could start immediately if
// the previous task(s) takes longer than the interval.
// Call Stop() on the return value in order to stop the ticker and to release associated resources.
// The interval must be greater than zero.
func Start(ctx context.Context, interval time.Duration, callback func(Tick), options ...Option) Stopper {
	t := &periodic{
		interval: interval,
		callback: callback,
	}

	for _, option := range options {
		option.apply(t)
	}

	ctx, cancelCtx := context.WithCancel(ctx)

	start := time.Now()

	go func() {
		done := false

		if !t.immediate {
			select {
			case <-time.After(interval):
			case <-ctx.Done():
				done = true
			}
		}

		if !done {
			ticker := time.NewTicker(t.interval)
			defer ticker.Stop()

			for tickTime := time.Now(); !done; {
				t.callback(Tick{
					Elapsed: tickTime.Sub(start),
					Time:    tickTime,
				})

				select {
				case tickTime = <-ticker.C:
				case <-ctx.Done():
					done = true
				}
			}
		}

		if t.onStop != nil {
			now := time.Now()
			t.onStop(Tick{
				Elapsed: now.Sub(start),
				Time:    now,
			})
		}
	}()

	return stoperFunc(func() {
		t.stop.Do(cancelCtx)
	})
}

type optionFunc func(*periodic)

func (f optionFunc) apply(p *periodic) {
	f(p)
}

type stoperFunc func()

func (f stoperFunc) Stop() {
	f()
}

type periodic struct {
	interval  time.Duration
	callback  func(Tick)
	immediate bool
	stop      sync.Once
	onStop    func(Tick)
}
