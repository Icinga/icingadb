package icingadb

import (
	"context"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"time"
)

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
