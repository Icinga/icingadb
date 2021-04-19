package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/icinga/icingadb/internal/command"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/flatten"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"runtime"
)

func main() {
	cmd := command.New()
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
	ha := icingadb.NewHA(ctx, db, heartbeat, logger)
	s := icingadb.NewSync(db, rc, logger)
	hs := history.NewSync(db, rc, logger)

	// For temporary exit after sync
	done := make(chan struct{}, 0)

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

						entities := delta.Create.Entities(synctx)
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
												CustomvarMeta: v1.CustomvarMeta{
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
													CustomvarId: customvar.Id,
												},
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
						v1.NewActionUrl,
						v1.NewCheckcommand,
						v1.NewCheckcommandArgument,
						v1.NewCheckcommandCustomvar,
						v1.NewCheckcommandEnvvar,
						v1.NewComment,
						v1.NewCustomvar,
						v1.NewDowntime,
						v1.NewEndpoint,
						v1.NewEventcommand,
						v1.NewEventcommandArgument,
						v1.NewEventcommandCustomvar,
						v1.NewEventcommandEnvvar,
						v1.NewHost,
						v1.NewHostCustomvar,
						v1.NewHostgroup,
						v1.NewHostgroupCustomvar,
						v1.NewHostgroupMember,
						v1.NewIconImage,
						v1.NewNotesUrl,
						v1.NewNotification,
						v1.NewNotificationcommand,
						v1.NewNotificationcommandArgument,
						v1.NewNotificationcommandCustomvar,
						v1.NewNotificationcommandEnvvar,
						v1.NewNotificationCustomvar,
						v1.NewNotificationRecipient,
						v1.NewNotificationUser,
						v1.NewNotificationUsergroup,
						v1.NewService,
						v1.NewServiceCustomvar,
						v1.NewServicegroup,
						v1.NewServicegroupCustomvar,
						v1.NewServicegroupMember,
						v1.NewTimeperiod,
						v1.NewTimeperiodCustomvar,
						v1.NewTimeperiodOverrideExclude,
						v1.NewTimeperiodOverrideInclude,
						v1.NewTimeperiodRange,
						v1.NewUser,
						v1.NewUserCustomvar,
						v1.NewUsergroup,
						v1.NewUsergroupCustomvar,
						v1.NewUsergroupMember,
						v1.NewZone,
					} {
						factoryFunc := factoryFunc

						ff := func() contracts.Entity {
							v := factoryFunc()
							if initer, ok := v.(contracts.Initer); ok {
								initer.Init()
							}

							return v
						}

						// vs
						// g.Go(func(factoryFunc contracts.EntityFactoryFunc) func() error {
						//     return func() error {
						//         return s.Sync(synctx, factoryFunc)
						//     }
						// }(factoryFunc))
						g.Go(func() error {
							return s.Sync(synctx, ff)
						})
					}

					g.Go(func() error {
						return hs.Sync(synctx)
					})

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
