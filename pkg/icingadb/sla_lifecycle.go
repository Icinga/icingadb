package icingadb

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"time"
)

// slaLifecycleTable defines the table name of v1.SlaLifecycle type.
var slaLifecycleTable = database.TableName(v1.NewSlaLifecycle())

// CreateSlaLifecyclesFromCheckables transforms the given checkables to sla lifecycle struct
// and streams them into a returned channel.
func CreateSlaLifecyclesFromCheckables(
	ctx context.Context, subject database.Entity, g *errgroup.Group, entities <-chan database.Entity, isDeleteEvent bool,
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

				sl := &v1.SlaLifecycle{
					EnvironmentMeta: v1.EnvironmentMeta{EnvironmentId: env.Id},
					CreateTime:      types.UnixMilli(time.Now()),
					DeleteTime:      types.UnixMilli(time.Unix(0, 0)),
				}

				if isDeleteEvent {
					sl.DeleteTime = types.UnixMilli(time.Now())
					sl.CreateTime = types.UnixMilli(time.Unix(0, 0))
				}

				switch subject.(type) {
				case *v1.Host:
					sl.Id = checkable.ID().(types.Binary)
					sl.HostId = sl.Id
				case *v1.Service:
					sl.Id = checkable.ID().(types.Binary)
					sl.ServiceId = sl.Id
					sl.HostId = checkable.(*v1.Service).HostId
				default:
					return errors.Errorf("sla lifecycle for type %T is not supported", checkable)
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

// SyncCheckablesSlaLifecycle inserts one `create_time` sla lifecycle entry for each of the checkables from
// the `host` and `service` tables and updates the `delete_time` of each of the sla lifecycle entries whose
// host/service IDs cannot be found in the `host/service` tables.
//
// It's unlikely, but when a given Checkable doesn't already have a `create_time` entry in the database, the update
// query won't update anything. Likewise, the insert statements may also become a no-op if the Checkables already
// have a `create_time` entry with ´delete_time = 0`.
//
// This function retries any database errors for at least `5m` before giving up and failing with an error.
func SyncCheckablesSlaLifecycle(ctx context.Context, db *database.DB) error {
	hostInsertStmtFmt := `
INSERT INTO %[1]s (id, environment_id, host_id, create_time)
  SELECT id, environment_id, id, %[2]d AS create_time
  FROM host WHERE NOT EXISTS(SELECT 1 FROM %[1]s WHERE service_id IS NULL AND delete_time = 0 AND host_id = host.id)`

	hostUpdateStmtFmt := `
UPDATE %[1]s SET delete_time = %[2]d
  WHERE service_id IS NULL AND delete_time = 0 AND NOT EXISTS(SELECT 1 FROM host WHERE host.id = %[1]s.id)`

	serviceInsertStmtFmt := `
INSERT INTO %[1]s (id, environment_id, host_id, service_id, create_time)
  SELECT id, environment_id, host_id, id, %[2]d AS create_time
  FROM service WHERE NOT EXISTS(SELECT 1 FROM %[1]s WHERE delete_time = 0 AND service_id = service.id)`

	serviceUpdateStmtFmt := `
UPDATE %[1]s SET delete_time = %[2]d
  WHERE delete_time = 0 AND service_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM service WHERE service.id = %[1]s.id)`

	return retry.WithBackoff(
		ctx,
		func(context.Context) error {
			eventTime := time.Now().UnixMilli()
			for _, queryFmt := range []string{hostInsertStmtFmt, hostUpdateStmtFmt, serviceInsertStmtFmt, serviceUpdateStmtFmt} {
				query := fmt.Sprintf(queryFmt, slaLifecycleTable, eventTime)
				if _, err := db.ExecContext(ctx, query); err != nil {
					return database.CantPerformQuery(err, query)
				}
			}

			return nil
		},
		retry.Retryable,
		backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second),
		db.GetDefaultRetrySettings(),
	)
}
