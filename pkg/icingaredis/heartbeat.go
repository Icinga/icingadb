package icingaredis

import (
	"context"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	v1 "github.com/icinga/icingadb/pkg/icingaredis/v1"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"sync"
	"sync/atomic"
	"time"
)

// Timeout defines how long a heartbeat may be absent if a heartbeat has already been received.
// After this time, a heartbeat loss is propagated.
const Timeout = time.Minute

// Heartbeat periodically reads heartbeats from a Redis stream and signals in Beat channels when they are received.
// Also signals on if the heartbeat is Lost.
type Heartbeat struct {
	active         bool
	events         chan *HeartbeatMessage
	lastReceivedMs atomic.Int64
	cancelCtx      context.CancelFunc
	client         *redis.Client
	done           chan struct{}
	errMu          sync.Mutex
	err            error
	logger         *logging.Logger
}

// NewHeartbeat returns a new Heartbeat and starts the heartbeat controller loop.
func NewHeartbeat(ctx context.Context, client *redis.Client, logger *logging.Logger) *Heartbeat {
	ctx, cancelCtx := context.WithCancel(ctx)

	heartbeat := &Heartbeat{
		events:    make(chan *HeartbeatMessage, 1),
		cancelCtx: cancelCtx,
		client:    client,
		done:      make(chan struct{}),
		logger:    logger,
	}

	go heartbeat.controller(ctx)

	return heartbeat
}

// Events returns a channel that is sent to on heartbeat events.
//
// A non-nil pointer signals that a heartbeat was received from Icinga 2 whereas a nil pointer signals a heartbeat loss.
func (h *Heartbeat) Events() <-chan *HeartbeatMessage {
	return h.events
}

// LastReceived returns the last heartbeat's receive time in ms.
func (h *Heartbeat) LastReceived() int64 {
	return h.lastReceivedMs.Load()
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

// controller loop.
func (h *Heartbeat) controller(ctx context.Context) {
	defer close(h.done)

	messages := make(chan *HeartbeatMessage)
	defer close(messages)

	g, ctx := errgroup.WithContext(ctx)

	// Message producer loop.
	g.Go(func() error {
		// We expect heartbeats every second but only read them every 3 seconds.
		throttle := time.NewTicker(3 * time.Second)
		defer throttle.Stop()

		for id := "$"; ; {
			streams, err := h.client.XReadUntilResult(ctx, &redis.XReadArgs{
				Streams: []string{"icinga:stats", id},
			})
			if err != nil {
				return errors.Wrap(err, "can't read Icinga heartbeat")
			}

			m := &HeartbeatMessage{
				received: time.Now(),
				stats:    streams[0].Messages[0].Values,
			}

			select {
			case messages <- m:
			case <-ctx.Done():
				return ctx.Err()
			}

			id = streams[0].Messages[0].ID

			<-throttle.C
		}
	})

	// State loop.
	g.Go(func() error {
		for {
			select {
			case m := <-messages:
				if !h.active {
					envId, err := m.EnvironmentID()
					if err != nil {
						return err
					}
					h.logger.Infow("Received Icinga heartbeat", zap.String("environment", envId.String()))
					h.active = true
				}

				h.lastReceivedMs.Store(m.received.UnixMilli())
				h.sendEvent(m)
			case <-time.After(Timeout):
				if h.active {
					h.logger.Warnw("Lost Icinga heartbeat", zap.Duration("timeout", Timeout))
					h.sendEvent(nil)
					h.active = false
				} else {
					h.logger.Warn("Waiting for Icinga heartbeat")
				}

				h.lastReceivedMs.Store(0)
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

func (h *Heartbeat) sendEvent(m *HeartbeatMessage) {
	// Remove any not yet delivered event
	select {
	case old := <-h.events:
		if old != nil {
			kv := []any{zap.Time("previous", old.received)}
			if m != nil {
				kv = append(kv, zap.Time("current", m.received))
			}

			h.logger.Debugw("Previous heartbeat not read from channel", kv...)
		} else {
			h.logger.Debug("Previous heartbeat loss event not read from channel")
		}
	default:
	}

	h.events <- m
}

// HeartbeatMessage represents a heartbeat received from Icinga 2 together with a timestamp when it was received.
type HeartbeatMessage struct {
	received time.Time
	stats    v1.StatsMessage
}

// Stats returns the underlying heartbeat message from the icinga:stats stream.
func (m *HeartbeatMessage) Stats() *v1.StatsMessage {
	return &m.stats
}

// EnvironmentID returns the Icinga DB environment ID stored in the heartbeat message.
func (m *HeartbeatMessage) EnvironmentID() (types.Binary, error) {
	var id types.Binary
	err := types.UnmarshalJSON([]byte(m.stats["icingadb_environment"].(string)), &id)
	if err != nil {
		return nil, err
	}
	return id, nil
}

// ExpiryTime returns the timestamp when the heartbeat expires.
func (m *HeartbeatMessage) ExpiryTime() time.Time {
	return m.received.Add(Timeout)
}
