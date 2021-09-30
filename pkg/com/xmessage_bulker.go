package com

import (
	"context"
	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

// XMessageBulker reads all values from a channel and streams them in chunks into a Bulk channel.
type XMessageBulker struct {
	ch  chan []redis.XMessage
	ctx context.Context
	mu  sync.Mutex
}

// NewXMessageBulker returns a new XMessageBulker and starts streaming.
func NewXMessageBulker(ctx context.Context, ch <-chan redis.XMessage, count int) *XMessageBulker {
	b := &XMessageBulker{
		ch:  make(chan []redis.XMessage),
		ctx: ctx,
		mu:  sync.Mutex{},
	}

	go b.run(ch, count)

	return b
}

// Bulk returns the channel on which the bulks are delivered.
func (b *XMessageBulker) Bulk() <-chan []redis.XMessage {
	return b.ch
}

func (b *XMessageBulker) run(ch <-chan redis.XMessage, count int) {
	defer close(b.ch)

	bufCh := make(chan redis.XMessage, count)
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
			buf := make([]redis.XMessage, 0, count)
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

// BulkXMessages reads all values from a channel and streams them in chunks into a returned channel.
func BulkXMessages(ctx context.Context, ch <-chan redis.XMessage, count int) <-chan []redis.XMessage {
	return NewXMessageBulker(ctx, ch, count).Bulk()
}
