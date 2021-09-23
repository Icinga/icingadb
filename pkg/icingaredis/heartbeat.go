package icingaredis

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/pkg/com"
	v1 "github.com/icinga/icingadb/pkg/icingaredis/v1"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

// timeout defines how long a heartbeat may be absent if a heartbeat has already been received.
// After this time, a heartbeat loss is propagated.
var timeout = 60 * time.Second

// Heartbeat periodically reads heartbeats from a Redis stream and signals in Beat channels when they are received.
// Also signals on if the heartbeat is Lost.
type Heartbeat struct {
	active    bool
	beat      *com.Cond
	cancelCtx context.CancelFunc
	client    *Client
	done      chan struct{}
	errMu     sync.Mutex
	err       error
	logger    *zap.SugaredLogger
	lost      *com.Cond
	message   *v1.StatsMessage
	messageMu sync.Mutex
}

// NewHeartbeat returns a new Heartbeat and starts the heartbeat controller loop.
func NewHeartbeat(ctx context.Context, client *Client, logger *zap.SugaredLogger) *Heartbeat {
	ctx, cancelCtx := context.WithCancel(ctx)

	heartbeat := &Heartbeat{
		beat:      com.NewCond(ctx),
		cancelCtx: cancelCtx,
		client:    client,
		done:      make(chan struct{}),
		logger:    logger,
		lost:      com.NewCond(ctx),
	}

	go heartbeat.controller(ctx)

	return heartbeat
}

// Beat returns a channel that will be closed when a new heartbeat is received.
func (h *Heartbeat) Beat() <-chan struct{} {
	return h.beat.Wait()
}

// Close stops the heartbeat controller loop, waits for it to finish, and returns an error if any.
// Implements the io.Closer interface.
func (h *Heartbeat) Close() error {
	h.cancelCtx()
	<-h.Done()

	return h.Err()
}

// Done returns a channel that will be closed when the heartbeat controller loop has ended.
func (h *Heartbeat) Done() <-chan struct{} {
	return h.done
}

// Err returns an error if Done has been closed and there is an error. Otherwise returns nil.
func (h *Heartbeat) Err() error {
	h.errMu.Lock()
	defer h.errMu.Unlock()

	return h.err
}

// Lost returns a channel that will be closed if the heartbeat is lost.
func (h *Heartbeat) Lost() <-chan struct{} {
	return h.lost.Wait()
}

// Message returns the last heartbeat message.
func (h *Heartbeat) Message() *v1.StatsMessage {
	h.messageMu.Lock()
	defer h.messageMu.Unlock()

	return h.message
}

// controller loop.
func (h *Heartbeat) controller(ctx context.Context) {
	defer close(h.done)

	h.logger.Info("Waiting for Icinga 2 heartbeat")

	messages := make(chan v1.StatsMessage)
	defer close(messages)

	g, ctx := errgroup.WithContext(ctx)

	// Message producer loop.
	g.Go(func() error {
		// We expect heartbeats every second but only read them every 3 seconds.
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

	// State loop.
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
				h.setMessage(&m)
				h.beat.Broadcast()
			case <-time.After(timeout):
				if h.active {
					h.logger.Warnw("Lost Icinga 2 heartbeat", zap.Duration("timeout", timeout))
					h.lost.Broadcast()
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
		// Do not propagate any context-canceled errors here,
		// as this is to be expected when calling Close or
		// when the parent context is canceled.
		h.setError(err)
	}
}

func (h *Heartbeat) setError(err error) {
	h.errMu.Lock()
	defer h.errMu.Unlock()

	h.err = errors.Wrap(err, "heartbeat failed")
}

func (h *Heartbeat) setMessage(m *v1.StatsMessage) {
	h.messageMu.Lock()
	defer h.messageMu.Unlock()

	h.message = m
}
