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
