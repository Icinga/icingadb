package icingadb

import (
	"context"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

type Delta struct {
	Create  EntitiesById
	Update  EntitiesById
	Delete  EntitiesById
	Subject *common.SyncSubject
	done    chan error
	err     error
	logger  *zap.SugaredLogger
}

func NewDelta(ctx context.Context, actual, desired <-chan contracts.Entity, subject *common.SyncSubject, logger *zap.SugaredLogger) *Delta {
	delta := &Delta{
		Subject: subject,
		done:    make(chan error, 1),
		logger:  logger,
	}

	go delta.start(ctx, actual, desired)

	return delta
}

func (delta *Delta) Wait() error {
	return <-delta.done
}

func (delta *Delta) start(ctx context.Context, actualCh, desiredCh <-chan contracts.Entity) {
	defer close(delta.done)

	var update EntitiesById
	if delta.Subject.WithChecksum() {
		update = EntitiesById{}
	}
	actual := EntitiesById{}
	desired := EntitiesById{}
	var mtx, updateMtx sync.Mutex
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var cnt com.Counter
		defer utils.Timed(time.Now(), func(elapsed time.Duration) {
			delta.logger.Debugf(
				"Synced %d actual elements of type %s in %s", cnt.Val(), utils.Name(delta.Subject.Entity()), elapsed)
		})
		for {
			select {
			case a, ok := <-actualCh:
				if !ok {
					return nil
				}

				id := a.ID().String()
				mtx.Lock()

				if d, ok := desired[id]; ok {
					delete(desired, id)
					mtx.Unlock()

					if delta.Subject.WithChecksum() && !a.(contracts.Checksumer).Checksum().Equal(d.(contracts.Checksumer).Checksum()) {
						updateMtx.Lock()
						update[id] = d
						updateMtx.Unlock()
					}
				} else {
					actual[id] = a
					mtx.Unlock()
				}

				cnt.Inc()
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	g.Go(func() error {
		var cnt com.Counter
		defer utils.Timed(time.Now(), func(elapsed time.Duration) {
			delta.logger.Debugf(
				"Synced %d desired elements of type %s in %s", cnt.Val(), utils.Name(delta.Subject.Entity()), elapsed)
		})
		for {
			select {
			case d, ok := <-desiredCh:
				if !ok {
					return nil
				}

				id := d.ID().String()
				mtx.Lock()

				if a, ok := actual[id]; ok {
					delete(actual, id)
					mtx.Unlock()

					if delta.Subject.WithChecksum() && !a.(contracts.Checksumer).Checksum().Equal(d.(contracts.Checksumer).Checksum()) {
						updateMtx.Lock()
						update[id] = d
						updateMtx.Unlock()
					}
				} else {
					desired[id] = d
					mtx.Unlock()
				}

				cnt.Inc()
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	if err := g.Wait(); err != nil {
		delta.done <- err

		return
	}

	delta.Create = desired
	delta.Delete = actual
	if delta.Subject.WithChecksum() {
		delta.Update = update
	}
}
