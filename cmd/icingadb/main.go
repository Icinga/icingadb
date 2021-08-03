package main

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/cmd/internal"
	"github.com/icinga/icingadb/internal/command"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/driver"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	"github.com/icinga/icingadb/pkg/icingadb/overdue"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/okzk/sdnotify"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	expectedRedisSchemaVersion    = "4"
	expectedMysqlSchemaVersion    = 3
	expectedPostgresSchemaVersion = 1
)

func main() {
	os.Exit(run())
}

func run() int {
	cmd := command.New()
	logs, err := logging.NewLogging(
		utils.AppName(),
		cmd.Config.Logging.Level,
		cmd.Config.Logging.Output,
		cmd.Config.Logging.Options,
		cmd.Config.Logging.Interval,
	)
	if err != nil {
		utils.Fatal(errors.Wrap(err, "can't configure logging"))
	}
	// When started by systemd, NOTIFY_SOCKET is set by systemd for Type=notify supervised services, which is the
	// default setting for the Icinga DB service. So we notify that Icinga DB finished starting up.
	_ = sdnotify.Ready()

	logger := logs.GetLogger()
	defer logger.Sync()

	logger.Info("Starting Icinga DB")

	db, err := cmd.Database(logs.GetChildLogger("database"))
	if err != nil {
		logger.Fatalf("%+v", errors.Wrap(err, "can't create database connection pool from config"))
	}
	defer db.Close()
	{
		logger.Info("Connecting to database")
		err := db.Ping()
		if err != nil {
			logger.Fatalf("%+v", errors.Wrap(err, "can't connect to database"))
		}
	}

	if err := checkDbSchema(context.Background(), db); err != nil {
		logger.Fatalf("%+v", err)
	}

	rc, err := cmd.Redis(logs.GetChildLogger("redis"))
	if err != nil {
		logger.Fatalf("%+v", errors.Wrap(err, "can't create Redis client from config"))
	}
	{
		logger.Info("Connecting to Redis")
		_, err := rc.Ping(context.Background()).Result()
		if err != nil {
			logger.Fatalf("%+v", errors.Wrap(err, "can't connect to Redis"))
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
	{
		rc, err := cmd.Redis(logs.GetChildLogger("redis"))
		if err != nil {
			logger.Fatalf("%+v", errors.Wrap(err, "can't create Redis client from config"))
		}
		heartbeat = icingaredis.NewHeartbeat(ctx, rc, logs.GetChildLogger("heartbeat"))

		db, err := cmd.Database(logs.GetChildLogger("database"))
		if err != nil {
			logger.Fatalf("%+v", errors.Wrap(err, "can't create database connection pool from config"))
		}
		defer db.Close()
		ha = icingadb.NewHA(ctx, db, heartbeat, logs.GetChildLogger("high-availability"))
	}
	// Closing ha on exit ensures that this instance retracts its heartbeat
	// from the database so that another instance can take over immediately.
	defer func() {
		// Give up after 3s, not 5m (default) not to hang for 5m if DB is down.
		ctx, cancelCtx := context.WithTimeout(context.Background(), 3*time.Second)

		ha.Close(ctx)
		cancelCtx()
	}()
	s := icingadb.NewSync(db, rc, logs.GetChildLogger("config-sync"))
	hs := history.NewSync(db, rc, logs.GetChildLogger("history-sync"))
	rt := icingadb.NewRuntimeUpdates(db, rc, logs.GetChildLogger("runtime-updates"))
	ods := overdue.NewSync(db, rc, logs.GetChildLogger("overdue-sync"))
	ret := history.NewRetention(
		db,
		cmd.Config.HistoryRetention.Days,
		cmd.Config.HistoryRetention.Interval,
		cmd.Config.HistoryRetention.Count,
		cmd.Config.HistoryRetention.Options,
		logs.GetChildLogger("history-retention"),
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
			case <-ha.Takeover():
				logger.Info("Taking over")

				go func() {
					for hactx.Err() == nil {
						synctx, cancelSynctx := context.WithCancel(ha.Environment().NewContext(hactx))
						g, synctx := errgroup.WithContext(synctx)
						// WaitGroups for initial synchronization.
						// Runtime updates must wait for initial synchronization to complete.
						configInitSync := sync.WaitGroup{}
						stateInitSync := &sync.WaitGroup{}

						// Get the last IDs of the runtime update streams before starting anything else,
						// otherwise updates may be lost.
						runtimeConfigUpdateStreams, runtimeStateUpdateStreams, err := rt.Streams(synctx)
						if err != nil {
							logger.Fatalf("%+v", err)
						}

						dump := icingadb.NewDumpSignals(rc, logs.GetChildLogger("dump-signals"))
						g.Go(func() error {
							logger.Debug("Staring config dump signal handling")

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
						logger.Info("Starting config sync")
						for _, factory := range v1.ConfigFactories {
							factory := factory

							configInitSync.Add(1)
							g.Go(func() error {
								defer configInitSync.Done()

								return s.SyncAfterDump(synctx, common.NewSyncSubject(factory), dump)
							})
						}
						logger.Info("Starting initial state sync")
						for _, factory := range v1.StateFactories {
							factory := factory

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

							elapsed := time.Since(syncStart)
							logger := logs.GetChildLogger("config-sync")
							if synctx.Err() == nil {
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
			case <-ha.Handover():
				logger.Warn("Handing over")

				cancelHactx()
			case <-hactx.Done():
				// Nothing to do here, surrounding loop will terminate now.
			case <-ha.Done():
				if err := ha.Err(); err != nil {
					logger.Fatalf("%+v", errors.Wrap(err, "HA exited with an error"))
				} else if ctx.Err() == nil {
					// ha is created as a single instance once. It should only exit if the main context is cancelled,
					// otherwise there is no way to get Icinga DB back into a working state.
					logger.Fatalf("%+v", errors.New("HA exited without an error but main context isn't cancelled"))
				}
				cancelHactx()

				return internal.ExitFailure
			case <-ctx.Done():
				logger.Fatalf("%+v", errors.New("main context closed unexpectedly"))
			case s := <-sig:
				logger.Infow("Exiting due to signal", zap.String("signal", s.String()))
				cancelHactx()

				return internal.ExitSuccess
			}
		}

		cancelHactx()
	}
}

// checkDbSchema asserts the database schema of the expected version being present.
func checkDbSchema(ctx context.Context, db *icingadb.DB) error {
	var expectedDbSchemaVersion uint16
	switch db.DriverName() {
	case driver.MySQL:
		expectedDbSchemaVersion = expectedMysqlSchemaVersion
	case driver.PostgreSQL:
		expectedDbSchemaVersion = expectedPostgresSchemaVersion
	}

	var version uint16

	err := db.QueryRowxContext(ctx, "SELECT version FROM icingadb_schema ORDER BY id DESC LIMIT 1").Scan(&version)
	if err != nil {
		return errors.Wrap(err, "can't check database schema version")
	}

	if version != expectedDbSchemaVersion {
		return errors.Errorf("expected database schema v%d, got v%d", expectedDbSchemaVersion, version)
	}

	return nil
}

// monitorRedisSchema monitors rc's icinga:schema version validity.
func monitorRedisSchema(logger *logging.Logger, rc *icingaredis.Client, pos string) {
	for {
		var err error
		pos, err = checkRedisSchema(logger, rc, pos)

		if err != nil {
			logger.Fatalf("%+v", err)
		}
	}
}

// checkRedisSchema verifies rc's icinga:schema version.
func checkRedisSchema(logger *logging.Logger, rc *icingaredis.Client, pos string) (newPos string, err error) {
	if pos == "0-0" {
		defer time.AfterFunc(3*time.Second, func() { logger.Info("Waiting for current Redis schema version") }).Stop()
	} else {
		logger.Debug("Waiting for new Redis schema version")
	}

	cmd := rc.XRead(context.Background(), &redis.XReadArgs{Streams: []string{"icinga:schema", pos}})
	xRead, err := cmd.Result()

	if err != nil {
		return "", icingaredis.WrapCmdErr(cmd)
	}

	message := xRead[0].Messages[0]
	if version := message.Values["version"]; version != expectedRedisSchemaVersion {
		return "", errors.Errorf(
			"unexpected Redis schema version: %q (expected %q)", version, expectedRedisSchemaVersion,
		)
	}

	logger.Debug("Redis schema version is correct")
	return message.ID, nil
}
