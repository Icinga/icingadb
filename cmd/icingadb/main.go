package main

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/icinga/icingadb/internal/command"
    "github.com/icinga/icingadb/pkg/contracts"
    "github.com/icinga/icingadb/pkg/flatten"
    "github.com/icinga/icingadb/pkg/icingadb"
    v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
    "github.com/icinga/icingadb/pkg/icingaredis"
    "github.com/icinga/icingadb/pkg/utils"
    "github.com/pkg/errors"
    "golang.org/x/sync/errgroup"
    "runtime"
)

func main() {
    cmd := command.New()
    instanceId := cmd.InstanceId()
    logger := cmd.Logger
    defer logger.Sync()
    defer func() {
        if err := recover(); err != nil {
            type stackTracer interface {
                StackTrace() errors.StackTrace
            }
            if err, ok := err.(stackTracer); ok {
                for _, f := range err.StackTrace() {
                    fmt.Printf("%+s:%d\n", f, f)
                }
            }
        }
    }()
    db := cmd.Database()
    defer db.Close()
    rc := cmd.Redis()

    ctx := context.Background()
    heartbeat := icingaredis.NewHeartbeat(ctx, rc, logger)
    ha := icingadb.NewHA(ctx, instanceId, db, heartbeat, logger)
    s := icingadb.NewSync(db, rc, logger)

    // For temporary exit after sync
    done := make(chan interface{}, 0)

    // Main loop
    for {
        hactx, cancel := context.WithCancel(ctx)
        for {
            select {
            case <-ha.Takeover():
                go func() {
                    g, synctx := errgroup.WithContext(hactx)

                    g.Go(func() error {
                        return nil
                        // TODO(el). This code is a draft for trying to synchronize customvar_flat from the customvar
                        // delta, which actually doesn't really make sense, since both synchronizations must always be
                        // completed without errors. The synchronization of customar and customvar_flat should only
                        // fetch the desired entities once and multiplex them to the synchronization of customvar and
                        // customvar_flat.
                        delta := s.GetDelta(synctx, v1.NewCustomvar)
                        if err := delta.Wait(); err != nil {
                            return err
                        }

                        entities := utils.SyncMapEntities(delta.Create)
                        flat := make(chan contracts.Entity, 0)

                        cvg, _ := errgroup.WithContext(synctx)

                        g.Go(func() error {
                            defer close(flat)

                            for i := 0; i < 1<<runtime.NumCPU()*2; i++ {
                                cvg.Go(func() error {
                                    for entity := range entities {
                                        var value interface{}
                                        customvar := entity.(*v1.Customvar)
                                        if err := json.Unmarshal([]byte(customvar.Value), &value); err != nil {
                                            return err
                                        }

                                        flattened := flatten.Flatten(value, customvar.Name)

                                        for flatname, flatvalue := range flattened {
                                            flatvalue := fmt.Sprintf("%v", flatvalue)
                                            flat <- &v1.CustomvarFlat{
                                                EntityWithoutChecksum: v1.EntityWithoutChecksum{
                                                    IdMeta: v1.IdMeta{
                                                        // TODO(el): Schema comment is wrong.
                                                        // Without customvar.Id we would produce duplicate keys here.
                                                        Id: utils.Checksum(customvar.EnvironmentId.String() + customvar.Id.String() + flatname + flatvalue),
                                                    },
                                                },
                                                EnvironmentMeta: v1.EnvironmentMeta{
                                                    EnvironmentId: customvar.EnvironmentId,
                                                },
                                                CustomvarId:      customvar.Id,
                                                Flatname:         flatname,
                                                FlatnameChecksum: utils.Checksum(flatname),
                                                Flatvalue:        flatvalue,
                                            }
                                        }
                                    }

                                    return nil
                                })
                            }

                            return cvg.Wait()
                        })

                        return db.Create(synctx, flat)
                    })

                    for _, factoryFunc := range []contracts.EntityFactoryFunc{
                        v1.NewCustomvar,
                        v1.NewHost,
                        v1.NewHostCustomvar,
                        v1.NewService,
                        v1.NewServiceCustomvar,
                    } {
                        factoryFunc := factoryFunc
                        // vs
                        // g.Go(func(factoryFunc contracts.EntityFactoryFunc) func() error {
                        //     return func() error {
                        //         return s.Sync(synctx, factoryFunc)
                        //     }
                        // }(factoryFunc))
                        g.Go(func() error {
                            return s.Sync(synctx, factoryFunc)
                        })
                    }

                    if err := g.Wait(); err != nil {
                        // TODO(el): This panics here even if a ctx gets cancelled.
                        // That is intentional for the moment for testing.
                        panic(err)
                    }

                    logger.Debugf("Requesting shutdown..")
                    close(done)
                }()
            case <-ha.Handover():
                cancel()
            case <-ctx.Done():
                if err := ctx.Err(); err != nil {
                    panic(err)
                }
            case <-done:
                return
            }
        }
    }
}
