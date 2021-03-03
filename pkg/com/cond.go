package com

import (
	"context"
	"errors"
)

var CondClosed = errors.New("condition closed")

// Cond implements a condition variable, a rendezvous point
// for goroutines waiting for or announcing the occurrence
// of an event.
type Cond struct {
	ctx       context.Context
	cancel    context.CancelFunc
	broadcast chan struct{}
	listeners chan chan struct{}
}

func NewCond(ctx context.Context) *Cond {
	done, cancel := context.WithCancel(ctx)

	c := &Cond{
		broadcast: make(chan struct{}),
		cancel:    cancel,
		ctx:       done,
		listeners: make(chan chan struct{}),
	}

	go c.controller()

	return c
}

// controller loop.
func (c *Cond) controller() {
	notify := make(chan struct{})

	for {
		select {
		case <-c.broadcast:
			// all current receivers get a closed channel
			close(notify)
			// set up next batch of receivers.
			notify = make(chan struct{})
		case c.listeners <- notify:
			// great. A Receiver has our channel
		case <-c.ctx.Done():
			return
		}
	}
}

// Close implements the io.Closer interface.
func (c *Cond) Close() error {
	c.cancel()

	return nil
}

// Wait returns a channel on which the next (close) signal will be sent.
func (c *Cond) Wait() <-chan struct{} {
	select {
	case l := <-c.listeners:
		return l
	case <-c.ctx.Done():
		panic(CondClosed)
	}
}

// Broadcast wakes all current listeners.
func (c *Cond) Broadcast() {
	select {
	case c.broadcast <- struct{}{}:
	case <-c.ctx.Done():
		panic(CondClosed)
	}
}

func (c *Cond) Done() <-chan struct{} {
	return c.ctx.Done()
}
