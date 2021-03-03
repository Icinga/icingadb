package icingaredis

import (
    "context"
    "encoding/json"
    "errors"
    "github.com/icinga/icingadb/pkg/com"
    "github.com/icinga/icingadb/pkg/contracts"
    "github.com/icinga/icingadb/pkg/types"
    "golang.org/x/sync/errgroup"
    "sync"
)

func CreateEntities(ctx context.Context, factoryFunc contracts.EntityFactoryFunc, pairs <-chan HPair, concurrent int) (<-chan contracts.Entity, <-chan error) {
    entities := make(chan contracts.Entity, 0)
    g, ctx := errgroup.WithContext(ctx)

    g.Go(func() error {
        defer close(entities)

        g, ctx := errgroup.WithContext(ctx)

        for i := 0; i < concurrent; i++ {
            g.Go(func() error {
                for pair := range pairs {
                    var id types.Binary

                    if err := id.UnmarshalText([]byte(pair.Field)); err != nil {
                        return err
                    }

                    e := factoryFunc()
                    if err := json.Unmarshal([]byte(pair.Value), e); err != nil {
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

func SetChecksums(ctx context.Context, entities <-chan contracts.Entity, checksums *sync.Map, concurrent int) (<-chan contracts.Entity, <-chan error) {
    entitiesWithChecksum := make(chan contracts.Entity, 0)
    g, ctx := errgroup.WithContext(ctx)

    g.Go(func() error {
        defer close(entitiesWithChecksum)

        g, ctx := errgroup.WithContext(ctx)

        for i := 0; i < concurrent; i++ {
            g.Go(func() error {
                for entity := range entities {
                    if checksumer, ok := checksums.Load(entity.ID().String()); ok {
                        entity.(contracts.Checksumer).SetChecksum(checksumer.(contracts.Checksumer).Checksum())
                    } else {
                        panic("no checksum")
                        // TODO(el): Error is not published
                        return errors.New("no checksum")
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
