package com

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"golang.org/x/sync/errgroup"
)

// WaitAsync calls Wait() on the passed Waiter in a new goroutine and
// sends the first non-nil error (if any) to the returned channel.
// The returned channel is always closed when the Waiter is done.
func WaitAsync(w contracts.Waiter) <-chan error {
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

// PipeError forwards the first non-nil error from in to out
// using a separate goroutine.
func PipeError(in <-chan error, out chan<- error) {
	go func() {
		if e := <-in; e != nil {
			out <- e
		}
	}()
}
