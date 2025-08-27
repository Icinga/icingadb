package main

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/internal/command"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	"github.com/icinga/icingadb/pkg/icingadb/overdue"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/icingaredis/telemetry"
	"github.com/okzk/sdnotify"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	ExitSuccess                = 0
	ExitFailure                = 1
	expectedRedisSchemaVersion = "6"
)

func main() {
	os.Exit(run())
}

func run() int {
	cmd := command.New()

	logs, err := logging.NewLoggingFromConfig(utils.AppName(), cmd.Config.Logging)
	if err != nil {
		utils.PrintErrorThenExit(err, ExitFailure)
	}

	// When started by systemd, NOTIFY_SOCKET is set by systemd for Type=notify supervised services, which is the
	// default setting for the Icinga DB service. So we notify that Icinga DB finished starting up.
	_ = sdnotify.Ready()

	logger := logs.GetLogger()
	defer func() { _ = logger.Sync() }()

	logger.WithOptions(logs.ForceLog()).Infof("Starting Icinga DB daemon (%s)", internal.Version.Version)

	db, err := cmd.Database(logs.GetChildLogger("database"))
	if err != nil {
		logger.Fatalw("Can't create database connection pool from config", zap.Error(err))
	}
	defer func() { _ = db.Close() }()
	{
		logger.Infof("Connecting to database at '%s'", db.GetAddr())
		err := db.Ping()
		if err != nil {
			logger.Fatalw("Can't connect to database", zap.Error(err))
		}
	}

	switch err := icingadb.CheckSchema(context.Background(), db); {
	case errors.Is(err, icingadb.ErrSchemaNotExists):
		if !cmd.Flags.DatabaseAutoImport {
			logger.Fatal("The database schema is missing")
		}

		logger.Info("Starting database schema auto import")
		if err := icingadb.ImportSchema(context.Background(), db, cmd.Flags.DatabaseSchemaDir); err != nil {
			logger.Fatalw("Can't import database schema", zap.Error(err))
		}
		logger.Info("The database schema was successfully imported")
	case err != nil:
		logger.Fatalf("%+v", err)
	}

	rc, err := cmd.Redis(logs.GetChildLogger("redis"))
	if err != nil {
		logger.Fatalw("Can't create Redis client from config", zap.Error(err))
	}
	{
		logger.Infof("Connecting to Redis at '%s'", rc.GetAddr())
		_, err := rc.Ping(context.Background()).Result()
		if err != nil {
			logger.Fatalw("Can't create Redis client from config", zap.Error(err))
		}
	}

	{
		pos, err := checkRedisSchema(logger, rc, "0-0")
		if err != nil {
			logger.Fatalf("%+v", err)
		}

		go monitorRedisSchema(logger, rc, pos)
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	// Use dedicated connections for heartbeat and HA to ensure that heartbeats are always processed and
	// the instance table is updated. Otherwise, the connections can be too busy due to the synchronization of
	// configuration, status, history, etc., which can lead to handover / takeover loops because
	// the heartbeat is not read while HA gets stuck when updating the instance table.
	var heartbeat *icingaredis.Heartbeat
	var ha *icingadb.HA
	var telemetrySyncStats *atomic.Pointer[telemetry.SuccessfulSync]
	{
		rc, err := cmd.Redis(logs.GetChildLogger("redis"))
		if err != nil {
			logger.Fatalw("Can't connect to Redis", zap.Error(err))
		}
		heartbeat = icingaredis.NewHeartbeat(ctx, rc, logs.GetChildLogger("heartbeat"))

		db, err := cmd.Database(logs.GetChildLogger("database"))
		if err != nil {
			logger.Fatalw("Can't create database connection pool from config", zap.Error(err))
		}
		defer func() { _ = db.Close() }()
		db.SetMaxOpenConns(1)
		ha = icingadb.NewHA(ctx, db, heartbeat, logs.GetChildLogger("high-availability"))

		telemetryLogger := logs.GetChildLogger("telemetry")
		telemetrySyncStats = telemetry.StartHeartbeat(ctx, rc, telemetryLogger, ha, heartbeat)
		telemetry.WriteStats(ctx, rc, telemetryLogger)
	}
	// Closing ha on exit ensures that this instance retracts its heartbeat
	// from the database so that another instance can take over immediately.
	defer func() {
		// Give up after 3s, not 5m (default) not to hang for 5m if DB is down.
		ctx, cancelCtx := context.WithTimeout(context.Background(), 3*time.Second)

		_ = ha.Close(ctx)
		cancelCtx()
	}()
	s := icingadb.NewSync(db, rc, logs.GetChildLogger("config-sync"))
	hs := history.NewSync(db, rc, logs.GetChildLogger("history-sync"))
	rt := icingadb.NewRuntimeUpdates(db, rc, logs.GetChildLogger("runtime-updates"))
	ods := overdue.NewSync(db, rc, logs.GetChildLogger("overdue-sync"))
	ret := history.NewRetention(
		db,
		cmd.Config.Retention.HistoryDays,
		cmd.Config.Retention.SlaDays,
		cmd.Config.Retention.Interval,
		cmd.Config.Retention.Count,
		cmd.Config.Retention.Options,
		logs.GetChildLogger("retention"),
	)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		logger.Info("Starting history sync")

		if err := hs.Sync(ctx); err != nil && !utils.IsContextCanceled(err) {
			logger.Fatalf("%+v", err)
		}
	}()

	// Main loop
	for {
		hactx, cancelHactx := context.WithCancel(ctx)
		for hactx.Err() == nil {
			select {
			case takeoverReason := <-ha.Takeover():
				logger.WithOptions(logs.ForceLog()).Infow("Taking over", zap.String("reason", takeoverReason))

				go func() {
					for hactx.Err() == nil {
						synctx, cancelSynctx := context.WithCancel(ha.Environment().NewContext(hactx))
						g, synctx := errgroup.WithContext(synctx)
						// WaitGroups for initial synchronization.
						// Runtime updates must wait for initial synchronization to complete.
						configInitSync := sync.WaitGroup{}
						stateInitSync := &sync.WaitGroup{}

						// Clear the runtime update streams before starting anything else (rather than after the sync),
						// otherwise updates may be lost.
						runtimeConfigUpdateStreams, runtimeStateUpdateStreams, err := rt.ClearStreams(synctx)
						if err != nil {
							logger.Fatalf("%+v", err)
						}

						dump := icingadb.NewDumpSignals(rc, logs.GetChildLogger("dump-signals"))
						g.Go(func() error {
							logger.Debug("Starting config dump signal handling")

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
							logger.Info("Starting overdue sync")

							return ods.Sync(synctx)
						})

						syncStart := time.Now()
						telemetry.OngoingSyncStartMilli.Store(syncStart.UnixMilli())

						logger.Info("Starting config sync")
						for _, factory := range v1.ConfigFactories {
							configInitSync.Add(1)
							g.Go(func() error {
								defer configInitSync.Done()

								return s.SyncAfterDump(synctx, common.NewSyncSubject(factory), dump)
							})
						}
						logger.Info("Starting initial state sync")
						for _, factory := range v1.StateFactories {
							stateInitSync.Add(1)
							g.Go(func() error {
								defer stateInitSync.Done()

								return s.SyncAfterDump(synctx, common.NewSyncSubject(factory), dump)
							})
						}

						configInitSync.Add(1)
						g.Go(func() error {
							defer configInitSync.Done()

							select {
							case <-dump.Done("icinga:customvar"):
							case <-synctx.Done():
								return synctx.Err()
							}

							return s.SyncCustomvars(synctx)
						})

						g.Go(func() error {
							configInitSync.Wait()
							telemetry.OngoingSyncStartMilli.Store(0)

							syncEnd := time.Now()
							elapsed := syncEnd.Sub(syncStart)
							logger := logs.GetChildLogger("config-sync")

							if synctx.Err() == nil {
								telemetrySyncStats.Store(&telemetry.SuccessfulSync{
									FinishMilli:   syncEnd.UnixMilli(),
									DurationMilli: elapsed.Milliseconds(),
								})

								logger.Infof("Finished config sync in %s", elapsed)
							} else {
								logger.Warnf("Aborted config sync after %s", elapsed)
							}

							return nil
						})

						g.Go(func() error {
							stateInitSync.Wait()

							elapsed := time.Since(syncStart)
							logger := logs.GetChildLogger("config-sync")
							if synctx.Err() == nil {
								logger.Infof("Finished initial state sync in %s", elapsed)
							} else {
								logger.Warnf("Aborted initial state sync after %s", elapsed)
							}

							return nil
						})

						g.Go(func() error {
							configInitSync.Wait()

							if err := synctx.Err(); err != nil {
								return err
							}

							logger.Info("Starting config runtime updates sync")

							return rt.Sync(synctx, v1.ConfigFactories, runtimeConfigUpdateStreams, false)
						})

						g.Go(func() error {
							stateInitSync.Wait()

							if err := synctx.Err(); err != nil {
								return err
							}

							logger.Info("Starting state runtime updates sync")

							return rt.Sync(synctx, v1.StateFactories, runtimeStateUpdateStreams, true)
						})

						g.Go(func() error {
							// Wait for config and state sync to avoid putting additional pressure on the database.
							configInitSync.Wait()
							stateInitSync.Wait()

							if err := synctx.Err(); err != nil {
								return err
							}

							logger.Info("Starting history retention")

							return ret.Start(synctx)
						})

						if err := g.Wait(); err != nil && !utils.IsContextCanceled(err) {
							logger.Fatalf("%+v", err)
						}
					}
				}()
			case handoverReason := <-ha.Handover():
				logger.WithOptions(logs.ForceLog()).Warnw("Handing over", zap.String("reason", handoverReason))

				cancelHactx()
			case <-hactx.Done():
				if ctx.Err() != nil {
					logger.Fatalw("Main context closed unexpectedly", zap.Error(ctx.Err()))
				}
				// Otherwise, there is nothing to do here, surrounding loop will terminate now.
			case <-ha.Done():
				if err := ha.Err(); err != nil {
					logger.Fatalw("HA exited with an error", zap.Error(err))
				} else if ctx.Err() == nil {
					// ha is created as a single instance once. It should only exit if the main context is cancelled,
					// otherwise there is no way to get Icinga DB back into a working state.
					logger.Fatal("HA exited without an error but main context isn't cancelled")
				}
				cancelHactx()

				return ExitFailure
			case s := <-sig:
				logger.Infow("Exiting due to signal", zap.String("signal", s.String()))
				cancelHactx()

				return ExitSuccess
			}
		}

		cancelHactx()
	}
}

// monitorRedisSchema monitors rc's icinga:schema version validity.
func monitorRedisSchema(logger *logging.Logger, rc *redis.Client, pos string) {
	for {
		var err error
		pos, err = checkRedisSchema(logger, rc, pos)

		if err != nil {
			logger.Fatalf("%+v", err)
		}
	}
}

// checkRedisSchema verifies rc's icinga:schema version.
func checkRedisSchema(logger *logging.Logger, rc *redis.Client, pos string) (newPos string, err error) {
	if pos == "0-0" {
		defer time.AfterFunc(3*time.Second, func() {
			logger.Info("Waiting for Icinga 2 to write into Redis, please make sure you have started Icinga 2 and the Icinga DB feature is enabled")
		}).Stop()
	} else {
		logger.Debug("Checking Icinga 2 and Icinga DB compatibility")
	}

	streams, err := rc.XReadUntilResult(context.Background(), &redis.XReadArgs{
		Streams: []string{"icinga:schema", pos},
	})
	if err != nil {
		return "", errors.Wrap(err, "can't read Redis schema version")
	}

	message := streams[0].Messages[0]
	if version := message.Values["version"]; version != expectedRedisSchemaVersion {
		// Since these error messages are trivial and mostly caused by users, we don't need
		// to print a stack trace here. However, since errors.Errorf() does this automatically,
		// we need to use fmt instead.
		return "", fmt.Errorf(
			"unexpected Redis schema version: %q (expected %q), please make sure you are running compatible"+
				" versions of Icinga 2 and Icinga DB", version, expectedRedisSchemaVersion,
		)
	}

	logger.Debug("Redis schema version is correct")
	return message.ID, nil
}
