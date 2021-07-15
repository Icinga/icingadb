package com

import (
	"context"
	"github.com/pkg/errors"
)

// Cond implements a channel-based synchronization for goroutines that wait for signals or send them.
// Internally based on a controller loop that handles the synchronization of new listeners and signal propagation,
// which is only started when NewCond is called. Thus the zero value cannot be used.
type Cond struct {
	broadcast chan struct{}
	done      chan struct{}
	cancel    context.CancelFunc
	listeners chan chan struct{}
}

// NewCond returns a new Cond and starts the controller loop.
func NewCond(ctx context.Context) *Cond {
	ctx, cancel := context.WithCancel(ctx)

	c := &Cond{
		broadcast: make(chan struct{}),
		cancel:    cancel,
		done:      make(chan struct{}),
		listeners: make(chan chan struct{}),
	}

	go c.controller(ctx)

	return c
}

// Broadcast sends a signal to all current listeners by closing the previously returned channel from Wait.
// Panics if the controller loop has already ended.
func (c *Cond) Broadcast() {
	select {
	case c.broadcast <- struct{}{}:
	case <-c.done:
		panic(errors.New("condition closed"))
	}
}

// Close stops the controller loop, waits for it to finish, and returns an error if any.
// Implements the io.Closer interface.
func (c *Cond) Close() error {
	c.cancel()
	<-c.done

	return nil
}

// Done returns a channel that will be closed when the controller loop has ended.
func (c *Cond) Done() <-chan struct{} {
	return c.done
}

// Wait returns a channel that is closed with the next signal.
// Panics if the controller loop has already ended.
func (c *Cond) Wait() <-chan struct{} {
	select {
	case l := <-c.listeners:
		return l
	case <-c.done:
		panic(errors.New("condition closed"))
	}
}

// controller loop.
func (c *Cond) controller(ctx context.Context) {
	defer close(c.done)

	// Note that the notify channel does not close when the controller loop ends
	// in order not to notify pending listeners.
	notify := make(chan struct{})

	for {
		select {
		case <-c.broadcast:
			// Close channel to notify all current listeners.
			close(notify)
			// Create a new channel for the next listeners.
			notify = make(chan struct{})
		case c.listeners <- notify:
			// A new listener received the channel.
		case <-ctx.Done():
			return
		}
	}
}
