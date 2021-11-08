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

// OnStop configures a callback that is executed when a periodic task is stopped or canceled.
func OnStop(f func(Tick)) Option {
	return optionFunc(func(p *periodic) {
		p.onStop = f
	})
}

// Start starts a periodic task with a ticker at the specified interval,
// which executes the given callback after each tick.
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
	ticker := time.NewTicker(t.interval)
	start := time.Now()

	go func() {
		defer ticker.Stop()

		for {
			select {
			case tickTime := <-ticker.C:
				t.callback(Tick{
					Elapsed: tickTime.Sub(start),
					Time:    tickTime,
				})
			case <-ctx.Done():
				if t.onStop != nil {
					now := time.Now()
					t.onStop(Tick{
						Elapsed: now.Sub(start),
						Time:    now,
					})
				}

				return
			}
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
	interval time.Duration
	callback func(Tick)
	stop     sync.Once
	onStop   func(Tick)
}
