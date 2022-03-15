package com

import (
	"context"
	"github.com/icinga/icingadb/pkg/contracts"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

// BulkChunkSplitPolicy is a state machine which tracks the items of a chunk a bulker assembles.
// A call takes an item for the current chunk into account.
// Output true indicates that the state machine was reset first and the bulker
// shall finish the current chunk now (not e.g. once $size is reached) without the given item.
type BulkChunkSplitPolicy func(contracts.Entity) bool

type BulkChunkSplitPolicyFactory func() BulkChunkSplitPolicy

// NeverSplit returns a pseudo state machine which never demands splitting.
func NeverSplit() BulkChunkSplitPolicy {
	return neverSplit
}

// SplitOnDupId returns a state machine which tracks the inputs' IDs.
// Once an already seen input arrives, it demands splitting.
func SplitOnDupId() BulkChunkSplitPolicy {
	seenIds := map[string]struct{}{}

	return func(entity contracts.Entity) bool {
		id := entity.ID().String()

		_, ok := seenIds[id]
		if ok {
			seenIds = map[string]struct{}{id: {}}
		} else {
			seenIds[id] = struct{}{}
		}

		return ok
	}
}

func neverSplit(contracts.Entity) bool {
	return false
}

// EntityBulker reads all entities from a channel and streams them in chunks into a Bulk channel.
type EntityBulker struct {
	ch  chan []contracts.Entity
	ctx context.Context
	mu  sync.Mutex
}

// NewEntityBulker returns a new EntityBulker and starts streaming.
func NewEntityBulker(
	ctx context.Context, ch <-chan contracts.Entity, count int, splitPolicyFactory BulkChunkSplitPolicyFactory,
) *EntityBulker {
	b := &EntityBulker{
		ch:  make(chan []contracts.Entity),
		ctx: ctx,
		mu:  sync.Mutex{},
	}

	go b.run(ch, count, splitPolicyFactory)

	return b
}

// Bulk returns the channel on which the bulks are delivered.
func (b *EntityBulker) Bulk() <-chan []contracts.Entity {
	return b.ch
}

func (b *EntityBulker) run(ch <-chan contracts.Entity, count int, splitPolicyFactory BulkChunkSplitPolicyFactory) {
	defer close(b.ch)

	bufCh := make(chan contracts.Entity, count)
	splitPolicy := splitPolicyFactory()
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

					if splitPolicy(v) {
						if len(buf) > 0 {
							b.ch <- buf
							buf = make([]contracts.Entity, 0, count)
						}

						timeout = time.After(256 * time.Millisecond)
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

			splitPolicy = splitPolicyFactory()
		}

		return nil
	})

	// We don't expect an error here.
	// We only use errgroup for the encapsulated use of sync.WaitGroup.
	_ = g.Wait()
}

// BulkEntities reads all entities from a channel and streams them in chunks into a returned channel.
func BulkEntities(
	ctx context.Context, ch <-chan contracts.Entity, count int, splitPolicyFactory BulkChunkSplitPolicyFactory,
) <-chan []contracts.Entity {
	if count <= 1 {
		return oneEntityBulk(ctx, ch)
	}

	return NewEntityBulker(ctx, ch, count, splitPolicyFactory).Bulk()
}

// oneEntityBulk operates just as NewEntityBulker(ctx, ch, 1, splitPolicy).Bulk(),
// but without the overhead of the actual bulk creation with a buffer channel, timeout and BulkChunkSplitPolicy.
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

var (
	_ BulkChunkSplitPolicyFactory = NeverSplit
	_ BulkChunkSplitPolicyFactory = SplitOnDupId
)
