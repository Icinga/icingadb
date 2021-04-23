package main

import (
	"context"
	"fmt"
	"github.com/icinga/icingadb/internal/command"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
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
