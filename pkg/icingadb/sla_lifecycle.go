package icingadb

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/types"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"time"
)

// tableName defines the table name of v1.SlaLifecycle type.
var tableName = database.TableName(v1.NewSlaLifecycle())

// GetCheckableFromSlaLifecycle returns the original checkable from which the specified sla lifecycle were transformed.
// When the passed entity is not of type *SlaLifecycle, it is returned as is.
func GetCheckableFromSlaLifecycle(e database.Entity) database.Entity {
	s, ok := e.(*v1.SlaLifecycle)
	if !ok {
		return e
	}

	return s.SourceEntity
}

// CreateSlaLifecyclesFromCheckables transforms the given checkables to sla lifecycle struct
// and streams them into a returned channel.
func CreateSlaLifecyclesFromCheckables(
	ctx context.Context, g *errgroup.Group, entities <-chan database.Entity, isDeleteEvent bool,
) <-chan database.Entity {
	slaLifecycles := make(chan database.Entity, 1)

	g.Go(func() error {
		defer close(slaLifecycles)

		env, ok := v1.EnvironmentFromContext(ctx)
		if !ok {
			return errors.New("can't get environment from context")
		}

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case checkable, ok := <-entities:
				if !ok {
					return nil
				}

				now := time.Now()

				sl := &v1.SlaLifecycle{
					EnvironmentMeta: v1.EnvironmentMeta{EnvironmentId: env.Id},
					CreateTime:      types.UnixMilli(now),
					DeleteTime:      types.UnixMilli(time.Unix(0, 0)),
					SourceEntity:    checkable,
				}

				switch checkable.(type) {
				case *v1.Host:
					sl.Id = checkable.ID().(types.Binary)
					sl.HostId = sl.Id
				case *v1.Service:
					sl.Id = checkable.ID().(types.Binary)
					sl.ServiceId = sl.Id
					if service, ok := checkable.(*v1.Service); ok {
						// checkable may be of type v1.EntityWithChecksum if this is a deletion event triggered
						// by the initial config sync as determined by the config delta calculation.
						sl.HostId = service.HostId
					}
				default:
					return errors.Errorf("sla lifecycle for type %T is not supported", checkable)
				}

				if isDeleteEvent {
					sl.DeleteTime = types.UnixMilli(now)
					sl.CreateTime = types.UnixMilli(time.Unix(0, 0))
				}

				select {
				case slaLifecycles <- sl:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	})

	return slaLifecycles
}

// StreamIDsFromUpdatedSlaLifecycles updates the `delete_time` of the sla lifecycle table for each of the Checkables
// consumed from the provided "entities" chan and upon successful execution of the query streams the original IDs
// of the entities into the returned channel.
//
// It's unlikely, but when a given Checkable doesn't already have a `create_time` entry in the database, the update
// query won't update anything. Either way the entities IDs are streamed into the returned chan.
func StreamIDsFromUpdatedSlaLifecycles(
	ctx context.Context, db *database.DB, g *errgroup.Group, logger *logging.Logger, entities <-chan database.Entity, bulkSize int,
) <-chan any {
	deleteEntityIDs := make(chan any, 1)

	g.Go(func() error {
		defer close(deleteEntityIDs)

		var counter com.Counter
		defer periodic.Start(ctx, logger.Interval(), func(_ periodic.Tick) {
			if count := counter.Reset(); count > 0 {
				logger.Infof("Updated %d sla lifecycles", count)
			}
		}).Stop()

		sem := db.GetSemaphoreForTable(tableName)
		stmt := fmt.Sprintf(`UPDATE %s SET delete_time = :delete_time WHERE "id" = :id AND "delete_time" = 0`, tableName)

		if bulkSize <= 0 {
			bulkSize = db.Options.MaxPlaceholdersPerStatement
		}

		// extractEntityId is used as a callback for the on success mechanism to extract the checkables id.
		extractEntityId := func(e database.Entity) any { return e.(*v1.SlaLifecycle).SourceEntity.ID() }

		return db.NamedBulkExec(
			ctx, stmt, bulkSize, sem, CreateSlaLifecyclesFromCheckables(ctx, g, entities, true),
			com.NeverSplit[database.Entity], OnSuccessApplyAndSendTo[database.Entity, any](deleteEntityIDs, extractEntityId))
	})

	return deleteEntityIDs
}
