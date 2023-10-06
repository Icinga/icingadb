package icingaredis

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/database"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// Streams represents a Redis stream key to ID mapping.
type Streams map[string]string

// Option returns the Redis stream key to ID mapping
// as a slice of stream keys followed by their IDs
// that is compatible for the Redis STREAMS option.
func (s Streams) Option() []string {
	// len*2 because we're appending the IDs later.
	streams := make([]string, 0, len(s)*2)
	ids := make([]string, 0, len(s))

	for key, id := range s {
		streams = append(streams, key)
		ids = append(ids, id)
	}

	return append(streams, ids...)
}

// CreateEntities streams and creates entities from the
// given Redis field value pairs using the specified factory function,
// and streams them on a returned channel.
func CreateEntities(ctx context.Context, factoryFunc database.EntityFactoryFunc, pairs <-chan HPair, concurrent int) (<-chan database.Entity, <-chan error) {
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

	return entities, com.WaitAsync(g)
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

	return entitiesWithChecksum, com.WaitAsync(g)
}

// WrapCmdErr adds the command itself and
// the stack of the current goroutine to the command's error if any.
func WrapCmdErr(cmd redis.Cmder) error {
	err := cmd.Err()
	if err != nil {
		err = errors.Wrapf(err, "can't perform %q", utils.Ellipsize(
			redis.NewCmd(context.Background(), cmd.Args()).String(), // Omits error in opposite to cmd.String()
			100,
		))
	}

	return err
}
