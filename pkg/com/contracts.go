package com

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
