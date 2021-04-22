package main

import (
	"context"
	"fmt"
	"github.com/icinga/icingadb/internal/command"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"os"
	"os/signal"
	"syscall"
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

	ctx, cancelCtx := context.WithCancel(context.Background())
	heartbeat := icingaredis.NewHeartbeat(ctx, rc, logger)
	ha := icingadb.NewHA(ctx, db, heartbeat, logger)
	defer ha.Close()
	s := icingadb.NewSync(db, rc, logger)
	hs := history.NewSync(db, rc, logger)

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Main loop
	for {
		hactx, cancel := context.WithCancel(ctx)
		for {
			select {
			case <-ha.Takeover():
				go func() {
					g, synctx := errgroup.WithContext(hactx)

					for _, factory := range v1.Factories {
						factory := factory

						g.Go(func() error {
							return s.Sync(synctx, factory.WithInit)
						})
					}

					g.Go(func() error {
						return hs.Sync(synctx)
					})

					if err := g.Wait(); err != nil && !utils.IsContextCanceled(err) {
						panic(err)
					}

					logger.Debugf("Requesting shutdown..")
					cancelCtx()
					return
				}()
			case <-ha.Handover():
				cancel()
			case <-ctx.Done():
				if err := ctx.Err(); err != nil && !utils.IsContextCanceled(err) {
					panic(err)
				}
			case s := <-sig:
				logger.Infow("Exiting due to signal", zap.String("signal", s.String()))
				cancelCtx()
				return
			}
		}
	}
}
