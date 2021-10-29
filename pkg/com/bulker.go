package com

import (
	"context"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

// Bulker reads all values from a channel and streams them in chunks into a Bulk channel.
type Bulker struct {
	ch  chan []interface{}
	ctx context.Context
	mu  sync.Mutex
}

// NewBulker returns a new Bulker and starts streaming.
func NewBulker(ctx context.Context, ch <-chan interface{}, count int) *Bulker {
	b := &Bulker{
		ch:  make(chan []interface{}),
		ctx: ctx,
		mu:  sync.Mutex{},
	}

	go b.run(ch, count)

	return b
}

// Bulk returns the channel on which the bulks are delivered.
func (b *Bulker) Bulk() <-chan []interface{} {
	return b.ch
}

func (b *Bulker) run(ch <-chan interface{}, count int) {
	defer close(b.ch)

	bufCh := make(chan interface{}, count)
	g, ctx := errgroup.WithContext(b.ctx)

	g.Go(func() error {
		defer close(bufCh)

		for {
			select {
			case v, ok := <-ch:
				if !ok {
					return nil
				}

				bufCh <- v
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	g.Go(func() error {
		for done := false; !done; {
			buf := make([]interface{}, 0, count)
			timeout := time.After(256 * time.Millisecond)

			for drain := true; drain && len(buf) < count; {
				select {
				case v, ok := <-bufCh:
					if !ok {
						drain = false
						done = true

						break
					}

					buf = append(buf, v)
				case <-timeout:
					drain = false
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			if len(buf) > 0 {
				b.ch <- buf
			}
		}

		return nil
	})

	// We don't expect an error here.
	// We only use errgroup for the encapsulated use of sync.WaitGroup.
	_ = g.Wait()
}

// Bulk reads all values from a channel and streams them in chunks into a returned channel.
func Bulk(ctx context.Context, ch <-chan interface{}, count int) <-chan []interface{} {
	if count <= 1 {
		return oneBulk(ctx, ch)
	}

	return NewBulker(ctx, ch, count).Bulk()
}

// oneBulk operates just as NewBulker(ctx, ch, 1).Bulk(),
// but without the overhead of the actual bulk creation with a buffer channel and timeout.
func oneBulk(ctx context.Context, ch <-chan interface{}) <-chan []interface{} {
	out := make(chan []interface{})
	go func() {
		defer close(out)

		for {
			select {
			case item := <-ch:
				select {
				case out <- []interface{}{item}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}
