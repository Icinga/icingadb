package com

import (
	"context"
	"github.com/icinga/icingadb/pkg/contracts"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

// EntityBulker reads all entities from a channel and streams them in chunks into a Bulk channel.
type EntityBulker struct {
	ch  chan []contracts.Entity
	ctx context.Context
	mu  sync.Mutex
}

// NewEntityBulker returns a new EntityBulker and starts streaming.
func NewEntityBulker(ctx context.Context, ch <-chan contracts.Entity, count int) *EntityBulker {
	b := &EntityBulker{
		ch:  make(chan []contracts.Entity),
		ctx: ctx,
		mu:  sync.Mutex{},
	}

	go b.run(ch, count)

	return b
}

// Bulk returns the channel on which the bulks are delivered.
func (b *EntityBulker) Bulk() <-chan []contracts.Entity {
	return b.ch
}

func (b *EntityBulker) run(ch <-chan contracts.Entity, count int) {
	defer close(b.ch)

	bufCh := make(chan contracts.Entity, count)
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
			buf := make([]contracts.Entity, 0, count)
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

// BulkEntities reads all entities from a channel and streams them in chunks into a returned channel.
func BulkEntities(ctx context.Context, ch <-chan contracts.Entity, count int) <-chan []contracts.Entity {
	if count <= 1 {
		return oneEntityBulk(ctx, ch)
	}

	return NewEntityBulker(ctx, ch, count).Bulk()
}

// oneEntityBulk operates just as NewEntityBulker(ctx, ch, 1).Bulk(),
// but without the overhead of the actual bulk creation with a buffer channel and timeout.
func oneEntityBulk(ctx context.Context, ch <-chan contracts.Entity) <-chan []contracts.Entity {
	out := make(chan []contracts.Entity)
	go func() {
		defer close(out)

		for {
			select {
			case item := <-ch:
				select {
				case out <- []contracts.Entity{item}:
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
