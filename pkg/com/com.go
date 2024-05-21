package com

import (
	"context"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// Waiter implements the Wait method,
// which blocks until execution is complete.
type Waiter interface {
	Wait() error // Wait waits for execution to complete.
}

// The WaiterFunc type is an adapter to allow the use of ordinary functions as Waiter.
// If f is a function with the appropriate signature, WaiterFunc(f) is a Waiter that calls f.
type WaiterFunc func() error

// Wait implements the Waiter interface.
func (f WaiterFunc) Wait() error {
	return f()
}

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
func CopyFirst[T any](
	ctx context.Context, input <-chan T,
) (first T, forward <-chan T, err error) {
	var ok bool
	select {
	case <-ctx.Done():
		var zero T

		return zero, nil, ctx.Err()
	case first, ok = <-input:
	}

	if !ok {
		err = errors.New("can't copy from closed channel")

		return
	}

	// Buffer of one because we receive an entity and send it back immediately.
	fwd := make(chan T, 1)
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
