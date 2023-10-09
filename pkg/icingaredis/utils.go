package icingaredis

import (
	"context"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/database"
	"github.com/icinga/icingadb/pkg/redis"
	"github.com/icinga/icingadb/pkg/strcase"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"runtime"
)

// CreateEntities streams and creates entities from the
// given Redis field value pairs using the specified factory function,
// and streams them on a returned channel.
func CreateEntities(ctx context.Context, factoryFunc database.EntityFactoryFunc, pairs <-chan redis.HPair, concurrent int) (<-chan database.Entity, <-chan error) {
	entities := make(chan database.Entity)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer close(entities)

		g, ctx := errgroup.WithContext(ctx)

		for i := 0; i < concurrent; i++ {
			g.Go(func() error {
				for pair := range pairs {
					var id types.Binary

					if err := id.UnmarshalText([]byte(pair.Field)); err != nil {
						return errors.Wrapf(err, "can't create ID from value %#v", pair.Field)
					}

					e := factoryFunc()
					if err := types.UnmarshalJSON([]byte(pair.Value), e); err != nil {
						return err
					}
					e.SetID(id)

					select {
					case entities <- e:
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				return nil
			})
		}

		return g.Wait()
	})

	return entities, com.WaitAsync(ctx, g)
}

// SetChecksums concurrently streams from the given entities and
// sets their checksums using the specified map and
// streams the results on a returned channel.
func SetChecksums(ctx context.Context, entities <-chan database.Entity, checksums map[string]database.Entity, concurrent int) (<-chan database.Entity, <-chan error) {
	entitiesWithChecksum := make(chan database.Entity)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer close(entitiesWithChecksum)

		g, ctx := errgroup.WithContext(ctx)

		for i := 0; i < concurrent; i++ {
			g.Go(func() error {
				for entity := range entities {
					if checksumer, ok := checksums[entity.ID().String()]; ok {
						entity.(contracts.Checksumer).SetChecksum(checksumer.(contracts.Checksumer).Checksum())
					} else {
						return errors.Errorf("no checksum for %#v", entity)
					}

					select {
					case entitiesWithChecksum <- entity:
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				return nil
			})
		}

		return g.Wait()
	})

	return entitiesWithChecksum, com.WaitAsync(ctx, g)
}

// YieldAll yields all entities from Redis that belong to the specified SyncSubject.
func YieldAll(ctx context.Context, c *redis.Client, subject *common.SyncSubject) (<-chan database.Entity, <-chan error) {
	key := strcase.Delimited(types.Name(subject.Entity()), ':')
	if subject.WithChecksum() {
		key = "icinga:checksum:" + key
	} else {
		key = "icinga:" + key
	}

	pairs, errs := c.HYield(ctx, key)
	g, ctx := errgroup.WithContext(ctx)
	// Let errors from HYield cancel the group.
	com.ErrgroupReceive(ctx, g, errs)

	desired, errs := CreateEntities(ctx, subject.FactoryForDelta(), pairs, runtime.NumCPU())
	// Let errors from CreateEntities cancel the group.
	com.ErrgroupReceive(ctx, g, errs)

	return desired, com.WaitAsync(ctx, g)
}
