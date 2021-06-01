package icingaredis

import (
	"context"
	"github.com/go-redis/redis/v8"
	v1 "github.com/icinga/icingadb/pkg/icingaredis/v1"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
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
	beat   chan v1.StatsMessage
	lost   chan struct{}
	done   chan struct{}
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
		beat:   make(chan v1.StatsMessage),
		lost:   make(chan struct{}),
		done:   make(chan struct{}),
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

func (h Heartbeat) Done() <-chan struct{} {
	return h.done
}

func (h Heartbeat) Err() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.err
}

func (h Heartbeat) Beat() <-chan v1.StatsMessage {
	return h.beat
}

func (h Heartbeat) Lost() <-chan struct{} {
	return h.lost
}

// controller loop.
func (h Heartbeat) controller() {
	messages := make(chan v1.StatsMessage)
	defer close(messages)

	g, ctx := errgroup.WithContext(h.ctx)

	// Message producer loop
	g.Go(func() error {
		// We expect heartbeats every second but only read them every 3 seconds
		throttle := time.NewTicker(time.Second * 3)
		defer throttle.Stop()

		for {
			cmd := h.client.XRead(ctx, &redis.XReadArgs{
				Streams: []string{"icinga:stats", "$"},
				Block:   0, // TODO(el): Might make sense to use a non-blocking variant here
			})

			streams, err := cmd.Result()
			if err != nil {
				return WrapCmdErr(cmd)
			}

			select {
			case messages <- streams[0].Messages[0].Values:
			case <-ctx.Done():
				return ctx.Err()
			}

			<-throttle.C
		}
	})

	// State loop
	g.Go(func() error {
		for {
			select {
			case m := <-messages:
				if !h.active {
					s, err := m.IcingaStatus()
					if err != nil {
						return errors.Wrapf(err, "can't parse Icinga 2 status from message %#v", m)
					}
					h.logger.Infow("Received first Icinga 2 heartbeat", zap.String("environment", s.Environment))
					h.active = true
				}
				h.beat <- m
			case <-time.After(timeout):
				if h.active {
					h.logger.Warn("Lost Icinga 2 heartbeat", zap.Duration("timeout", timeout))
					h.lost <- struct{}{}
					h.active = false
				} else {
					h.logger.Warn("Waiting for Icinga 2 heartbeat")
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
	h.err = errors.Wrap(err, "heartbeat failed")
	h.mu.Unlock()
}
