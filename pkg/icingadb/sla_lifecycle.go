package icingadb

import (
	"context"
	"fmt"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/icingadb/objectpacker"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"time"
)

var tableName = utils.TableName(v1.NewSlaLifecycle())

// GetCheckableFromSlaLifecycle returns the original checkable from which the specified sla lifecycle were transformed.
// When the passed entity is not of type *SlaLifecycle, it is returned as is.
func GetCheckableFromSlaLifecycle(e contracts.Entity) contracts.Entity {
	s, ok := e.(*v1.SlaLifecycle)
	if !ok {
		return e
	}

	return s.SourceEntity
}

// CreateSlaLifecyclesFromCheckables transforms the given checkables to sla lifecycle struct
// and streams them into a returned channel.
func CreateSlaLifecyclesFromCheckables(
	ctx context.Context, g *errgroup.Group, db *DB, entities <-chan contracts.Entity, isDeleteEvent bool,
) <-chan contracts.Entity {
	slaLifecycles := make(chan contracts.Entity, 1)

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
					sl.HostId = checkable.(v1.BinaryIDer).BinaryID()
					sl.Id = utils.Checksum(objectpacker.MustPackSlice(sl.EnvironmentId, sl.HostId))
				case *v1.Service:
					sl.HostId = checkable.(v1.BinaryHostIDer).BinaryHostID()
					sl.ServiceId = checkable.(v1.BinaryIDer).BinaryID()
					if !sl.HostId.Valid() {
						// This is only the case for services that are deleted at runtime, as Icinga 2 does not include
						// the `host_id` in the stream. So consider dropping this piece of code once Icinga 2 decides
						// to include this data in the redis `delete` stream.
						err := db.QueryRowxContext(
							ctx, db.Rebind(`SELECT "host_id" FROM "service" WHERE "id" = ?`), sl.ServiceId,
						).StructScan(sl)

						if err != nil {
							return err
						}
					}

					sl.Id = utils.Checksum(objectpacker.MustPackSlice(sl.EnvironmentId, sl.HostId, sl.ServiceId))
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

// UpdateSlaLifecycles updates the `delete_time` of the sla lifecycle for the given checkables.
// When a given checkable doesn't already have a `create_time` entry in the database, the update query won't
// update anything. Either way the sla lifecycle entries are streamed into the returned chan.
func UpdateSlaLifecycles(
	ctx context.Context, db *DB, entities <-chan contracts.Entity, g *errgroup.Group, bulkSize int, bufferLen int, onSuccess ...OnSuccess[contracts.Entity],
) <-chan contracts.Entity {
	updatedSlaLifeCycles := make(chan contracts.Entity, bufferLen)

	g.Go(func() error {
		defer close(updatedSlaLifeCycles)

		sem := db.GetSemaphoreForTable(tableName)
		stmt := fmt.Sprintf(`UPDATE %s SET delete_time = :delete_time WHERE "id" = :id AND "delete_time" = 0`, tableName)

		if bulkSize <= 0 {
			bulkSize = db.Options.MaxPlaceholdersPerStatement
		}

		onSuccess := append(onSuccess[:len(onSuccess):len(onSuccess)], OnSuccessSendTo(updatedSlaLifeCycles))

		return db.NamedBulkExec(
			ctx, stmt, bulkSize, sem, CreateSlaLifecyclesFromCheckables(ctx, g, db, entities, true),
			com.NeverSplit[contracts.Entity], onSuccess...,
		)
	})

	return updatedSlaLifeCycles
}
