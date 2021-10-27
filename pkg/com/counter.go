package com

import (
	"sync"
	"sync/atomic"
)

// Counter implements an atomic counter.
type Counter struct {
	value uint64
	mu    sync.Mutex // Protects total.
	total uint64
}

// Add adds the given delta to the counter.
func (c *Counter) Add(delta uint64) {
	atomic.AddUint64(&c.value, delta)
}

// Inc increments the counter by one.
func (c *Counter) Inc() {
	c.Add(1)
}

// Reset resets the counter to 0 and returns its previous value.
// Does not reset the total value returned from Total.
func (c *Counter) Reset() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	v := atomic.SwapUint64(&c.value, 0)
	c.total += v

	return v
}

// Total returns the total counter value.
func (c *Counter) Total() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.total + c.Val()
}

// Val returns the current counter value.
func (c *Counter) Val() uint64 {
	return atomic.LoadUint64(&c.value)
}
