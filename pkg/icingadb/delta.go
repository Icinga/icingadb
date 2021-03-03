package icingadb

import (
	"context"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

type Delta struct {
	Create       *sync.Map
	Update       *sync.Map
	Delete       *sync.Map
	WithChecksum bool
	done         chan error
	err          error
	logger       *zap.SugaredLogger
}

func NewDelta(ctx context.Context, actual, desired <-chan contracts.Entity, withChecksum bool, logger *zap.SugaredLogger) *Delta {
	delta := &Delta{
		WithChecksum: withChecksum,
		done:         make(chan error, 1),
		logger:       logger,
	}

	go delta.start(ctx, actual, desired)

	return delta
}

func (delta Delta) Wait() error {
	return <-delta.done
}

func (delta *Delta) start(ctx context.Context, actualCh, desiredCh <-chan contracts.Entity) {
	defer close(delta.done)

	var update sync.Map
	if delta.WithChecksum {
		update = sync.Map{}
	}
	actual := sync.Map{}
	desired := sync.Map{}
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var cnt com.Counter
		defer utils.Timed(time.Now(), func(elapsed time.Duration) {
			delta.logger.Debugf("Synced %d actual elements in %s", cnt.Val(), elapsed)
		})
		for {
			select {
			case a, ok := <-actualCh:
				if !ok {
					return nil
				}

				id := a.ID().String()
				if d, ok := desired.Load(id); ok {
					desired.Delete(id)

					if delta.WithChecksum && !a.(contracts.Checksumer).Checksum().Equal(d.(contracts.Checksumer).Checksum()) {
						update.Store(id, d)
					}
				} else {
					actual.Store(id, a)
				}

				cnt.Inc()
			case <-ctx.Done():
				return nil
			}
		}
	})

	g.Go(func() error {
		var cnt com.Counter
		defer utils.Timed(time.Now(), func(elapsed time.Duration) {
			delta.logger.Debugf("Synced %d desired elements in %s", cnt.Val(), elapsed)
		})
		for {
			select {
			case d, ok := <-desiredCh:
				if !ok {
					return nil
				}

				id := d.ID().String()
				if a, ok := actual.Load(id); ok {
					actual.Delete(id)

					if delta.WithChecksum && !a.(contracts.Checksumer).Checksum().Equal(d.(contracts.Checksumer).Checksum()) {
						update.Store(id, d)
					}
				} else {
					desired.Store(id, d)
				}

				cnt.Inc()
			case <-ctx.Done():
				return nil
			}
		}
	})

	if err := g.Wait(); err != nil {
		delta.done <- err

		return
	}

	delta.Create = &desired
	delta.Delete = &actual
	if delta.WithChecksum {
		delta.Update = &update
	}
}
