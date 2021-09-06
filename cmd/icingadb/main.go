package main

import (
	"context"
	"github.com/icinga/icingadb/internal/command"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	"github.com/icinga/icingadb/pkg/icingadb/overdue"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const (
	ExitSuccess           = 0
	ExitFailure           = 1
	expectedSchemaVersion = 2
)

func main() {
	os.Exit(run())
}

func run() int {
	cmd := command.New()
	// logging instance
	logging := cmd.Logging()

	// default logger
	logger := logging.GetLogger()
	defer logger.Sync()

	logger.Info("Starting Icinga DB")

	db, err := cmd.Database(logging.GetChildLogger("database"))
	if err != nil {
		logging.Fatalf("%+v", errors.Wrap(err, "can't connect to database"))
	}

	defer db.Close()
	{
		logger.Info("Connecting to database")
		err := db.Ping()
		if err != nil {
			logging.Fatalf("%+v", errors.Wrap(err, "can't connect to database"))
		}
	}

	if err := checkDbSchema(context.Background(), db); err != nil {
		logging.Fatalf("%+v", err)
	}

	rc, err := cmd.Redis(logging.GetChildLogger("redis"))
	if err != nil {
		logging.Fatalf("%+v", errors.Wrap(err, "can't connect to database"))
	}

	{
		logger.Info("Connecting to Redis")
		_, err := rc.Ping(context.Background()).Result()
		if err != nil {
			logging.Fatalf("%+v", errors.Wrap(err, "can't create Redis client from config"))
		}
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	heartbeat := icingaredis.NewHeartbeat(ctx, rc, logging.GetChildLogger("heartbeat"))

	ha := icingadb.NewHA(ctx, db, heartbeat, logging.GetChildLogger("ha"))
	// Closing ha on exit ensures that this instance retracts its heartbeat
	// from the database so that another instance can take over immediately.
	defer ha.Close()
	s := icingadb.NewSync(db, rc, logging.GetChildLogger("sync"))
	hs := history.NewSync(db, rc, logging.GetChildLogger("history"))
	rt := icingadb.NewRuntimeUpdates(db, rc, logging.GetChildLogger("runtime-update"))
	ods := overdue.NewSync(db, rc, logging.GetChildLogger("overdue"))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Main loop
	for {
		hactx, cancelHactx := context.WithCancel(ctx)
		for hactx.Err() == nil {
			select {
			case <-ha.Takeover():
				logger.Info("Taking over")

				go func() {
					for hactx.Err() == nil {
						synctx, cancelSynctx := context.WithCancel(hactx)
						g, synctx := errgroup.WithContext(synctx)
						// WaitGroup for configuration synchronization.
						// Runtime updates must wait for configuration synchronization to complete.
						wg := sync.WaitGroup{}

						// Get the last IDs of the runtime update streams before starting anything else,
						// otherwise updates may be lost.
						runtimeUpdateStreams, err := rt.Streams(ctx)
						if err != nil {
							logger.Fatalf("%+v", err)
						}

						dump := icingadb.NewDumpSignals(rc, logging.GetChildLogger("dump-signals"))
						g.Go(func() error {
							logger.Info("Staring config dump signal handling")

							return dump.Listen(synctx)
						})

						g.Go(func() error {
							select {
							case <-dump.InProgress():
								logger.Info("Icinga 2 started a new config dump, waiting for it to complete")
								cancelSynctx()

								return nil
							case <-synctx.Done():
								return synctx.Err()
							}
						})

						g.Go(func() error {
							logger.Info("Starting history sync")

							return hs.Sync(synctx)
						})

						g.Go(func() error {
							logger.Info("Starting overdue sync")

							return ods.Sync(synctx)
						})

						logger.Info("Starting config sync")
						for _, factory := range v1.Factories {
							factory := factory

							wg.Add(1)
							g.Go(func() error {
								defer wg.Done()

								return s.SyncAfterDump(synctx, common.NewSyncSubject(factory.WithInit), dump)
							})
						}

						wg.Add(1)
						g.Go(func() error {
							defer wg.Done()

							<-dump.Done("icinga:customvar")

							logger.Infof("Syncing customvar")

							cv := common.NewSyncSubject(v1.NewCustomvar)

							cvs, redisErrs := rc.YieldAll(synctx, cv)
							// Let errors from Redis cancel our group.
							com.ErrgroupReceive(g, redisErrs)

							// Multiplex cvs to use them both for customvar and customvar_flat.
							cvs1, cvs2 := make(chan contracts.Entity), make(chan contracts.Entity)
							g.Go(func() error {
								defer close(cvs1)
								defer close(cvs2)
								for {
									select {
									case cv, ok := <-cvs:
										if !ok {
											return nil
										}

										cvs1 <- cv
										cvs2 <- cv
									case <-synctx.Done():
										return synctx.Err()
									}
								}
							})

							actualCvs, dbErrs := db.YieldAll(
								ctx, cv.Factory(), db.BuildSelectStmt(cv.Entity(), cv.Entity().Fingerprint()))
							// Let errors from DB cancel our group.
							com.ErrgroupReceive(g, dbErrs)

							g.Go(func() error {
								return s.ApplyDelta(ctx, icingadb.NewDelta(ctx, actualCvs, cvs1, cv, logging.GetChildLogger("delta")))
							})

							cvFlat := common.NewSyncSubject(v1.NewCustomvarFlat)

							cvFlats, flattenErrs := v1.FlattenCustomvars(ctx, cvs2)
							// Let errors from Flatten cancel our group.
							com.ErrgroupReceive(g, flattenErrs)

							actualCvFlats, dbErrs := db.YieldAll(
								ctx, cvFlat.Factory(), db.BuildSelectStmt(cvFlat.Entity(), cvFlat.Entity().Fingerprint()))
							// Let errors from DB cancel our group.
							com.ErrgroupReceive(g, dbErrs)

							g.Go(func() error {
								return s.ApplyDelta(ctx, icingadb.NewDelta(ctx, actualCvFlats, cvFlats, cvFlat, logging.GetChildLogger("delta")))
							})

							return nil
						})

						g.Go(func() error {
							wg.Wait()

							logger.Info("Starting runtime updates sync")

							// @TODO(el): The customvar runtime update sync may change because the customvar flat
							// runtime update sync is not yet implemented.
							return rt.Sync(
								synctx,
								append([]contracts.EntityFactoryFunc{v1.NewCustomvar}, v1.Factories...),
								runtimeUpdateStreams,
							)
						})

						if err := g.Wait(); err != nil && !utils.IsContextCanceled(err) {
							logging.Fatalf("%+v", err)
						}
					}
				}()
			case <-ha.Handover():
				logger.Warn("Handing over")

				cancelHactx()
			case <-hactx.Done():
				// Nothing to do here, surrounding loop will terminate now.
			case <-ha.Done():
				if err := ha.Err(); err != nil {
					logging.Fatalf("%+v", errors.Wrap(err, "HA exited with an error"))
				} else if ctx.Err() == nil {
					// ha is created as a single instance once. It should only exit if the main context is cancelled,
					// otherwise there is no way to get Icinga DB back into a working state.
					logging.Fatalf("%+v", errors.New("HA exited without an error but main context isn't cancelled"))
				}
				cancelHactx()

				return ExitFailure
			case <-ctx.Done():
				logging.Fatalf("%+v", errors.New("main context closed unexpectedly"))
			case s := <-sig:
				logger.Infow("Exiting due to signal", zap.String("signal", s.String()))
				cancelHactx()

				return ExitSuccess
			}
		}

		cancelHactx()
	}
}

// checkDbSchema asserts the database schema of the expected version being present.
func checkDbSchema(ctx context.Context, db *icingadb.DB) error {
	var version uint16

	err := db.QueryRowxContext(ctx, "SELECT version FROM icingadb_schema ORDER BY id DESC LIMIT 1").Scan(&version)
	if err != nil {
		return errors.Wrap(err, "can't check database schema version")
	}

	if version != expectedSchemaVersion {
		return errors.Errorf("expected database schema v%d, got v%d", expectedSchemaVersion, version)
	}

	return nil
}
