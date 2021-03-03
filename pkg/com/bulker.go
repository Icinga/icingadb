package com

import (
	"context"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

type Bulker struct {
	ch  chan []interface{}
	ctx context.Context
	mu  sync.Mutex
	err error
}

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
				return nil
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
					return nil
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

func Bulk(ctx context.Context, ch <-chan interface{}, count int) <-chan []interface{} {
	return NewBulker(ctx, ch, count).Bulk()
}
