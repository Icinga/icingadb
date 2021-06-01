package icingadb

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"sync"
)

type DumpSignals struct {
	redis        *icingaredis.Client
	logger       *zap.SugaredLogger
	mutex        sync.Mutex
	doneCh       map[string]chan struct{}
	allDoneCh    chan struct{}
	inProgressCh chan struct{}
}

func NewDumpSignals(redis *icingaredis.Client, logger *zap.SugaredLogger) *DumpSignals {
	return &DumpSignals{
		redis:        redis,
		logger:       logger,
		doneCh:       make(map[string]chan struct{}),
		inProgressCh: make(chan struct{}),
	}
}

// Listen starts listening for dump signals in the icinga:dump Redis stream. When a done signal is received, this is
// signaled via the channels returned from the Done function.
//
// If a wip signal is received after a done signal was passed on via the Done function, this is signaled via the
// InProgress function and this function returns with err == nil. In this case, all other signals are invalidated.
// It is up to the caller to pass on this information, for example by cancelling derived contexts.
//
// This function may only be called once for each DumpSignals object. To listen for a new iteration of dump signals, a new
// DumpSignals instance must be created.
func (s *DumpSignals) Listen(ctx context.Context) error {
	// Closing a channel twice results in a panic. This function takes a chan struct{} and closes it unless it is
	// already closed. In this case it just does nothing. This function assumes that the channel is never written to
	// and that there are no concurrent attempts to close the channel.
	safeClose := func(ch chan struct{}) {
		select {
		case <-ch:
			// Reading from a closed channel returns immediately, therefore don't close it again.
		default:
			close(ch)
		}
	}

	lastStreamId := "0-0"
	anyDoneSent := false

	for {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "can't listen for dump signals")
		}

		cmd := s.redis.XRead(ctx, &redis.XReadArgs{
			Streams: []string{"icinga:dump", lastStreamId},
			Block:   0, // block indefinitely
		})
		result, err := cmd.Result()
		if err != nil {
			return icingaredis.WrapCmdErr(cmd)
		}

		for _, entry := range result[0].Messages {
			lastStreamId = entry.ID
			key := entry.Values["key"].(string)
			done := entry.Values["state"].(string) == "done"

			s.logger.Debugw("Received dump signal from Redis", zap.String("key", key), zap.Bool("done", done))

			if done {
				if key == "*" {
					if s.allDoneCh == nil {
						s.mutex.Lock()

						// Set s.allDoneCh to signal for all future listeners that we've received an all-done signal.
						s.allDoneCh = make(chan struct{})
						close(s.allDoneCh)

						// Notify all existing listeners.
						for _, ch := range s.doneCh {
							safeClose(ch)
						}

						s.mutex.Unlock()
					}
				} else {
					s.mutex.Lock()
					if ch, ok := s.doneCh[key]; ok {
						safeClose(ch)
					}
					s.mutex.Unlock()
				}
				anyDoneSent = true
			} else if anyDoneSent {
				// Received a wip signal after handing out any done signal via one of the channels returned by Done,
				// signal that a new dump is in progress. This treats every state=wip as if it has key=*, which is the
				// only key for which state=wip is currently sent by Icinga 2.
				close(s.inProgressCh)
				return nil
			}
		}
	}
}

// Done returns a channel that is closed when the given key receives a done dump signal.
func (s *DumpSignals) Done(key string) <-chan struct{} {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.allDoneCh != nil {
		// An all done-signal was already received, don't care about individual key anymore.
		return s.allDoneCh
	} else if ch, ok := s.doneCh[key]; ok {
		// Return existing wait channel for this key.
		return ch
	} else {
		// First request for this key, create new wait channel.
		ch = make(chan struct{})
		s.doneCh[key] = ch
		return ch
	}
}

// InProgress returns a channel that is closed when a new dump is in progress after done signals were sent to channels
// returned by Done.
func (s *DumpSignals) InProgress() <-chan struct{} {
	return s.inProgressCh
}
