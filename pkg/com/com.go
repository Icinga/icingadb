package com

import (
	"context"
	"github.com/icinga/icingadb/pkg/database"
	"golang.org/x/sync/errgroup"
)

// WaitAsync calls Wait() on the passed Waiter in a new goroutine and
// sends the first non-nil error (if any) to the returned channel.
// The returned channel is always closed when the Waiter is done.
func WaitAsync(w Waiter) <-chan error {
	errs := make(chan error, 1)

	go func() {
		defer close(errs)

		if e := w.Wait(); e != nil {
			errs <- e
		}
	}()

	return errs
}

// ErrgroupReceive adds a goroutine to the specified group that
// returns the first non-nil error (if any) from the specified channel.
// If the channel is closed, it will return nil.
func ErrgroupReceive(g *errgroup.Group, err <-chan error) {
	g.Go(func() error {
		if e := <-err; e != nil {
			return e
		}

		return nil
	})
}

// CopyFirst asynchronously forwards all items from input to forward and synchronously returns the first item.
func CopyFirst(
	ctx context.Context, input <-chan database.Entity,
) (first database.Entity, forward <-chan database.Entity, err error) {
	var ok bool
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case first, ok = <-input:
	}

	if !ok {
		return
	}

	// Buffer of one because we receive an entity and send it back immediately.
	fwd := make(chan database.Entity, 1)
	fwd <- first

	forward = fwd

	go func() {
		defer close(fwd)

		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-input:
				if !ok {
					return
				}

				select {
				case <-ctx.Done():
					return
				case fwd <- e:
				}
			}
		}
	}()

	return
}
