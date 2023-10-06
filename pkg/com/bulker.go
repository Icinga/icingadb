package com

import (
	"context"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

// BulkChunkSplitPolicy is a state machine which tracks the items of a chunk a bulker assembles.
// A call takes an item for the current chunk into account.
// Output true indicates that the state machine was reset first and the bulker
// shall finish the current chunk now (not e.g. once $size is reached) without the given item.
type BulkChunkSplitPolicy[T any] func(T) bool

type BulkChunkSplitPolicyFactory[T any] func() BulkChunkSplitPolicy[T]

// NeverSplit returns a pseudo state machine which never demands splitting.
func NeverSplit[T any]() BulkChunkSplitPolicy[T] {
	return neverSplit[T]
}

func neverSplit[T any](T) bool {
	return false
}

// Bulker reads all values from a channel and streams them in chunks into a Bulk channel.
type Bulker[T any] struct {
	ch  chan []T
	ctx context.Context
	mu  sync.Mutex
}

// NewBulker returns a new Bulker and starts streaming.
func NewBulker[T any](
	ctx context.Context, ch <-chan T, count int, splitPolicyFactory BulkChunkSplitPolicyFactory[T],
) *Bulker[T] {
	b := &Bulker[T]{
		ch:  make(chan []T),
		ctx: ctx,
		mu:  sync.Mutex{},
	}

	go b.run(ch, count, splitPolicyFactory)

	return b
}

// Bulk returns the channel on which the bulks are delivered.
func (b *Bulker[T]) Bulk() <-chan []T {
	return b.ch
}

func (b *Bulker[T]) run(ch <-chan T, count int, splitPolicyFactory BulkChunkSplitPolicyFactory[T]) {
	defer close(b.ch)

	bufCh := make(chan T, count)
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
			buf := make([]T, 0, count)
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
							buf = make([]T, 0, count)
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

// Bulk reads all values from a channel and streams them in chunks into a returned channel.
func Bulk[T any](
	ctx context.Context, ch <-chan T, count int, splitPolicyFactory BulkChunkSplitPolicyFactory[T],
) <-chan []T {
	if count <= 1 {
		return oneBulk(ctx, ch)
	}

	return NewBulker(ctx, ch, count, splitPolicyFactory).Bulk()
}

// oneBulk operates just as NewBulker(ctx, ch, 1, splitPolicy).Bulk(),
// but without the overhead of the actual bulk creation with a buffer channel, timeout and BulkChunkSplitPolicy.
func oneBulk[T any](ctx context.Context, ch <-chan T) <-chan []T {
	out := make(chan []T)
	go func() {
		defer close(out)

		for {
			select {
			case item, ok := <-ch:
				if !ok {
					return
				}

				select {
				case out <- []T{item}:
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
	_ BulkChunkSplitPolicyFactory[struct{}] = NeverSplit[struct{}]
)
