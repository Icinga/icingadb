package icingaredis

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func CreateEntities(ctx context.Context, factoryFunc contracts.EntityFactoryFunc, pairs <-chan HPair, concurrent int) (<-chan contracts.Entity, <-chan error) {
	entities := make(chan contracts.Entity)
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
					if err := internal.UnmarshalJSON([]byte(pair.Value), e); err != nil {
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

func SetChecksums(ctx context.Context, entities <-chan contracts.Entity, checksums map[string]contracts.Entity, concurrent int) (<-chan contracts.Entity, <-chan error) {
	entitiesWithChecksum := make(chan contracts.Entity)
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
						panic("no checksum")
						// TODO(el): Error is not published
						//return errors.New("no checksum")
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

// WrapCmdErr adds the command itself and the stack of the current goroutine to the command's error if any.
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
