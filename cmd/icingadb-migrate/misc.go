package main

import (
	"context"
	"crypto/sha1"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/driver"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/objectpacker"
	icingadbTypes "github.com/icinga/icingadb/pkg/types"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/vbauerster/mpb/v6"
	"github.com/vbauerster/mpb/v6/decor"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"strings"
	"time"
)

type IdoMigrationProgressUpserter struct {
	LastIdoId any `json:"last_ido_id"`
}

// Upsert implements the contracts.Upserter interface.
func (impu *IdoMigrationProgressUpserter) Upsert() interface{} {
	return impu
}

type IdoMigrationProgress struct {
	IdoMigrationProgressUpserter `json:",inline"`
	EnvironmentId                string `json:"environment_id"`
	HistoryType                  string `json:"history_type"`
	FromTs                       int32  `json:"from_ts"`
	ToTs                         int32  `json:"to_ts"`
}

// Assert interface compliance.
var (
	_ contracts.Upserter = (*IdoMigrationProgressUpserter)(nil)
	_ contracts.Upserter = (*IdoMigrationProgress)(nil)
)

// log is the root logger.
var log = func() *zap.SugaredLogger {
	logger, err := zap.NewDevelopmentConfig().Build()
	if err != nil {
		panic(err)
	}

	return logger.Sugar()
}()

// objectTypes maps IDO values to Icinga DB ones.
var objectTypes = map[uint8]string{1: "host", 2: "service"}

// hashAny combines objectpacker.PackAny and SHA1 hashing.
func hashAny(in interface{}) []byte {
	hash := sha1.New()
	if err := objectpacker.PackAny(in, hash); err != nil {
		panic(err)
	}

	return hash.Sum(nil)
}

// convertTime converts *nix timestamps from the IDO for Icinga DB.
func convertTime(ts int64, tsUs uint32) icingadbTypes.UnixMilli {
	if ts == 0 && tsUs == 0 {
		return icingadbTypes.UnixMilli{}
	}

	return icingadbTypes.UnixMilli(time.Unix(ts, int64(tsUs)*int64(time.Microsecond/time.Nanosecond)))
}

// calcObjectId calculates the ID of the config object named name1 for Icinga DB.
func calcObjectId(env, name1 string) []byte {
	if name1 == "" {
		return nil
	}

	return hashAny([2]string{env, name1})
}

// calcServiceId calculates the ID of the service name2 of the host name1 for Icinga DB.
func calcServiceId(env, name1, name2 string) []byte {
	if name2 == "" {
		return nil
	}

	return hashAny([2]string{env, name1 + "!" + name2})
}

// sliceIdoHistory performs query with args+fromid,toid,checkpoint,bulk on ht.snapshot
// and passes the results to onRows until either an empty result set or onRows() returns nil.
// Rationale: split the likely large result set of a query by adding a WHERE condition and a LIMIT,
// both with :named placeholders (:checkpoint, :bulk).
// checkpoint is the initial value for the WHERE condition, onRows() returns follow-up ones.
// (On non-recoverable errors the whole program exits.)
func sliceIdoHistory[Row any](
	ht *historyType, query string, args map[string]any,
	checkpoint interface{}, onRows func([]Row) (checkpoint interface{}),
) {
	if args == nil {
		args = map[string]interface{}{}
	}

	args["fromid"] = ht.fromId
	args["toid"] = ht.toId
	args["checkpoint"] = checkpoint
	args["bulk"] = 20000

	if ht.snapshot.DriverName() != driver.MySQL {
		query = strings.ReplaceAll(query, " USE INDEX (PRIMARY)", "")
	}

	for {
		// TODO: use Tx#SelectNamed() one nice day (https://github.com/jmoiron/sqlx/issues/779)
		stmt, err := ht.snapshot.PrepareNamed(query)
		if err != nil {
			log.With("query", query).Fatalf("%+v", errors.Wrap(err, "can't prepare query"))
		}

		var rows []Row
		if err := stmt.Select(&rows, args); err != nil {
			log.With("query", query).Fatalf("%+v", errors.Wrap(err, "can't perform query"))
		}

		_ = stmt.Close()

		if len(rows) < 1 {
			break
		}

		if checkpoint = onRows(rows); checkpoint == nil {
			break
		}

		args["checkpoint"] = checkpoint
	}
}

type progressBar struct {
	*mpb.Bar

	lastUpdate time.Time
}

// IncrBy does pb.Bar.DecoratorEwmaUpdate() automatically.
func (pb *progressBar) IncrBy(n int) {
	pb.Bar.IncrBy(n)

	now := time.Now()

	if !pb.lastUpdate.IsZero() {
		pb.Bar.DecoratorEwmaUpdate(now.Sub(pb.lastUpdate))
	}

	pb.lastUpdate = now
}

// historyType specifies a history data type.
type historyType struct {
	// name is a human-readable common name.
	name string
	// idoTable specifies the source table.
	idoTable string
	// idoIdColumn specifies idoTable's primary key.
	idoIdColumn string
	// idoStartColumns specifies idoTable's event start time locations. (First non-NULL is used.)
	idoStartColumns []string
	// idoEndColumns specifies idoTable's event end time locations. (First non-NULL is used.)
	idoEndColumns []string
	// cacheSchema specifies <name>.sqlite3's structure.
	cacheSchema string
	// cacheFiller fills cache from snapshot.
	cacheFiller func(*historyType)
	// cacheLimitQuery rationale: see migrate().
	cacheLimitQuery string
	// migrationQuery SELECTs source data for actual migration.
	migrationQuery string
	// migrate does the actual migration.
	migrate func(c *Config, idb *icingadb.DB, envId []byte, ht *historyType)

	// cacheFile locates <name>.sqlite3.
	cacheFile string
	// cache represents <cacheFile>.
	cache *sqlx.DB
	// snapshot represents the data source.
	snapshot *sqlx.Tx
	// fromId is the first IDO row ID to migrate.
	fromId uint64
	// toId is the last IDO row ID to migrate.
	toId uint64
	// total summarizes the source data.
	total int64
	// cacheTotal summarizes the cache source data.
	cacheTotal int64
	// done summarizes the migrated data.
	done int64
	// bar represents the current progress bar.
	bar *progressBar
	// lastId is the last already migrated ID.
	lastId uint64
}

// setupBar (re-)initializes ht.bar.
func (ht *historyType) setupBar(progress *mpb.Progress, total int64) {
	ht.bar = &progressBar{Bar: progress.AddBar(
		total,
		mpb.BarFillerClearOnComplete(),
		mpb.PrependDecorators(
			decor.Name(ht.name, decor.WC{W: len(ht.name) + 1, C: decor.DidentRight}),
			decor.Percentage(decor.WC{W: 5}),
		),
		mpb.AppendDecorators(
			decor.EwmaETA(decor.ET_STYLE_GO, 0, decor.WC{W: 4}),
			decor.Name(" "),
			decor.EwmaSpeed(0, "%.0f/s", 0, decor.WC{W: 4}),
		),
	)}
}

type historyTypes []*historyType

// forEach performs f per hts in parallel.
func (hts historyTypes) forEach(f func(*historyType)) {
	eg, _ := errgroup.WithContext(context.Background())
	for _, ht := range hts {
		ht := ht
		eg.Go(func() error {
			f(ht)
			return nil
		})
	}

	_ = eg.Wait()
}

type icingaDbOutputStage struct {
	insert, upsert []contracts.Entity
}

var types = historyTypes{
	{
		name:            "ack & comment",
		idoTable:        "icinga_commenthistory",
		idoIdColumn:     "commenthistory_id",
		idoStartColumns: []string{"entry_time"},
		// Manual deletion time wins vs. time of expiration which never happens due to manual deletion.
		idoEndColumns:  []string{"deletion_time", "expiration_time"},
		migrationQuery: commentMigrationQuery,
		migrate: func(c *Config, idb *icingadb.DB, envId []byte, ht *historyType) {
			migrateOneType(c, idb, envId, ht, convertCommentRows)
		},
	},
	{
		name:        "downtime",
		idoTable:    "icinga_downtimehistory",
		idoIdColumn: "downtimehistory_id",
		// Fall back to scheduled time if actual time is missing.
		idoStartColumns: []string{"actual_start_time", "scheduled_start_time"},
		idoEndColumns:   []string{"actual_end_time", "scheduled_end_time"},
		migrationQuery:  downtimeMigrationQuery,
		migrate: func(c *Config, idb *icingadb.DB, envId []byte, ht *historyType) {
			migrateOneType(c, idb, envId, ht, convertDowntimeRows)
		},
	},
	{
		name:            "flapping",
		idoTable:        "icinga_flappinghistory",
		idoIdColumn:     "flappinghistory_id",
		idoStartColumns: []string{"event_time"},
		idoEndColumns:   []string{"event_time"},
		cacheSchema:     eventTimeCacheSchema,
		cacheFiller: func(ht *historyType) {
			buildEventTimeCache(ht, []string{
				"xh.flappinghistory_id id", "UNIX_TIMESTAMP(xh.event_time) event_time",
				"xh.event_time_usec", "1001-xh.event_type event_is_start", "xh.object_id",
			})
		},
		migrationQuery: flappingMigrationQuery,
		migrate: func(c *Config, idb *icingadb.DB, envId []byte, ht *historyType) {
			migrateOneType(c, idb, envId, ht, convertFlappingRows)
		},
	},
	{
		name:            "notification",
		idoTable:        "icinga_notifications",
		idoIdColumn:     "notification_id",
		idoStartColumns: []string{"start_time"},
		idoEndColumns:   []string{"end_time"},
		cacheSchema:     previousHardStateCacheSchema,
		cacheFiller: func(ht *historyType) {
			buildPreviousHardStateCache(ht, []string{
				"xh.notification_id id", "xh.object_id", "xh.state last_hard_state",
			})
		},
		cacheLimitQuery: "SELECT MAX(history_id) FROM previous_hard_state",
		migrationQuery:  notificationMigrationQuery,
		migrate: func(c *Config, idb *icingadb.DB, envId []byte, ht *historyType) {
			migrateOneType(c, idb, envId, ht, convertNotificationRows)
		},
	},
	{
		name:            "state",
		idoTable:        "icinga_statehistory",
		idoIdColumn:     "statehistory_id",
		idoStartColumns: []string{"state_time"},
		idoEndColumns:   []string{"state_time"},
		cacheSchema:     previousHardStateCacheSchema,
		cacheFiller: func(ht *historyType) {
			buildPreviousHardStateCache(ht, []string{"xh.statehistory_id id", "xh.object_id", "xh.last_hard_state"})
		},
		cacheLimitQuery: "SELECT MAX(history_id) FROM previous_hard_state",
		migrationQuery:  stateMigrationQuery,
		migrate: func(c *Config, idb *icingadb.DB, envId []byte, ht *historyType) {
			migrateOneType(c, idb, envId, ht, convertStateRows)
		},
	},
}
