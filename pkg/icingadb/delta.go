package icingadb

import (
	"context"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/utils"
	"go.uber.org/zap"
	"time"
)

// Delta calculates the delta of actual and desired entities, and stores which entities need to be created, updated, and deleted.
type Delta struct {
	Create  EntitiesById
	Update  EntitiesById
	Delete  EntitiesById
	Subject *common.SyncSubject
	done    chan error
	logger  *zap.SugaredLogger
}

// NewDelta creates a new Delta and starts calculating it. The caller must ensure
// that no duplicate entities are sent to the same stream.
func NewDelta(ctx context.Context, actual, desired <-chan contracts.Entity, subject *common.SyncSubject, logger *zap.SugaredLogger) *Delta {
	delta := &Delta{
		Subject: subject,
		done:    make(chan error, 1),
		logger:  logger,
	}

	go delta.run(ctx, actual, desired)

	return delta
}

// Wait waits for the delta calculation to complete and returns an error, if any.
func (delta *Delta) Wait() error {
	return <-delta.done
}

func (delta *Delta) run(ctx context.Context, actualCh, desiredCh <-chan contracts.Entity) {
	defer close(delta.done)

	start := time.Now()
	var endActual, endDesired time.Time
	var numActual, numDesired uint64

	actual := EntitiesById{}  // only read from actualCh (so far)
	desired := EntitiesById{} // only read from desiredCh (so far)

	var update EntitiesById
	if delta.Subject.WithChecksum() {
		update = EntitiesById{} // read from actualCh and desiredCh with mismatching checksums
	}

	for actualCh != nil || desiredCh != nil {
		select {
		case actualValue, ok := <-actualCh:
			if !ok {
				endActual = time.Now()
				actualCh = nil // Done reading all actual entities, disable this case.
				break
			}
			numActual++

			id := actualValue.ID().String()
			if desiredValue, ok := desired[id]; ok {
				delete(desired, id)
				if update != nil && !checksumsMatch(actualValue, desiredValue) {
					update[id] = desiredValue
				}
			} else {
				actual[id] = actualValue
			}

		case desiredValue, ok := <-desiredCh:
			if !ok {
				endDesired = time.Now()
				desiredCh = nil // Done reading all desired entities, disable this case.
				break
			}
			numDesired++

			id := desiredValue.ID().String()
			if actualValue, ok := actual[id]; ok {
				delete(actual, id)
				if update != nil && !checksumsMatch(actualValue, desiredValue) {
					update[id] = desiredValue
				}
			} else {
				desired[id] = desiredValue
			}

		case <-ctx.Done():
			delta.done <- ctx.Err()
			return
		}
	}

	delta.Create = desired
	delta.Update = update
	delta.Delete = actual

	delta.logger.Debugw("Delta finished",
		zap.String("subject", utils.Name(delta.Subject.Entity())),
		zap.Duration("time_total", time.Since(start)),
		zap.Duration("time_actual", endActual.Sub(start)),
		zap.Duration("time_desired", endDesired.Sub(start)),
		zap.Uint64("num_actual", numActual),
		zap.Uint64("num_desired", numDesired),
		zap.Int("create", len(delta.Create)),
		zap.Int("update", len(delta.Update)),
		zap.Int("delete", len(delta.Delete)))
}

// checksumsMatch returns whether the checksums of two entities are the same.
// Both entities must implement contracts.Checksumer.
func checksumsMatch(a, b contracts.Entity) bool {
	c1 := a.(contracts.Checksumer).Checksum()
	c2 := b.(contracts.Checksumer).Checksum()
	return c1.Equal(c2)
}
