package com

import "sync/atomic"

// Counter implements an atomic counter.
type Counter uint64

// Add adds the given delta to the counter.
func (c *Counter) Add(delta uint64) {
	atomic.AddUint64(c.ptr(), delta)
}

// Inc increments the counter by one.
func (c *Counter) Inc() {
	c.Add(1)
}

// Val returns the counter value.
func (c *Counter) Val() uint64 {
	return atomic.LoadUint64(c.ptr())
}

func (c *Counter) ptr() *uint64 {
	return (*uint64)(c)
}
