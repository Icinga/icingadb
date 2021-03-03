package icingaredis

import (
	"context"
	"github.com/go-redis/redis/v8"
	v1 "github.com/icinga/icingadb/pkg/icingaredis/v1"
	"github.com/icinga/icingadb/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

var timeout = 60 * time.Second

type Heartbeat struct {
	ctx    context.Context
	cancel context.CancelFunc
	client *Client
	logger *zap.SugaredLogger
	active bool
	beat   chan interface{}
	lost   chan interface{}
	done   chan interface{}
	mu     *sync.Mutex
	err    error
}

func NewHeartbeat(ctx context.Context, client *Client, logger *zap.SugaredLogger) *Heartbeat {
	ctx, cancel := context.WithCancel(ctx)

	heartbeat := &Heartbeat{
		ctx:    ctx,
		cancel: cancel,
		client: client,
		logger: logger,
		beat:   make(chan interface{}),
		lost:   make(chan interface{}),
		done:   make(chan interface{}),
		mu:     &sync.Mutex{},
	}

	go heartbeat.controller()

	return heartbeat
}

// Close implements the io.Closer interface.
func (h Heartbeat) Close() error {
	// Cancel ctx.
	h.cancel()
	// Wait until the controller loop ended.
	<-h.Done()
	// And return an error, if any.
	return h.Err()
}

func (h Heartbeat) Done() <-chan interface{} {
	return h.done
}

func (h Heartbeat) Err() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.err
}

func (h Heartbeat) Beat() <-chan interface{} {
	return h.beat
}

func (h Heartbeat) Lost() <-chan interface{} {
	return h.lost
}

// controller loop.
func (h Heartbeat) controller() {
	messages := make(chan interface{})
	defer close(messages)

	g, ctx := errgroup.WithContext(h.ctx)

	// Message producer loop
	g.Go(func() error {
		// We expect heartbeats every second but only read them every 3 seconds
		throttle := time.Tick(time.Second * 3)
		for {
			streams, err := h.client.XRead(ctx, &redis.XReadArgs{
				Streams: []string{"icinga:stats", "$"},
				Block:   0, // TODO(el): Might make sense to use a non-blocking variant here
			}).Result()
			if err != nil {
				return err
			}

			select {
			case messages <- v1.StatsMessage(streams[0].Messages[0].Values):
			case <-ctx.Done():
				return ctx.Err()
			}

			<-throttle
		}
	})

	// State loop
	g.Go(func() error {
		for {
			select {
			case m := <-messages:
				h.active = true // TODO(el): We might only want to set this once
				h.beat <- m
			case <-time.After(timeout):
				if h.active {
					h.lost <- struct{}{}
					h.active = false
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	// Since the goroutines of the group actually run endlessly,
	// we wait here forever, unless an error occurs.
	if err := g.Wait(); err != nil && !utils.IsContextCanceled(err) {
		// Do not propagate context-aborted errors here,
		// as this is to be expected when Close was called.
		h.setError(err)
	}
}

func (h *Heartbeat) setError(err error) {
	h.mu.Lock()
	h.err = err
	h.mu.Unlock()
}
