package main

import (
	"context"
	"crypto/sha1"
	"github.com/icinga/icingadb/pkg/icingadb/objectpacker"
	icingadbTypes "github.com/icinga/icingadb/pkg/types"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/vbauerster/mpb/v6"
	"github.com/vbauerster/mpb/v6/decor"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"reflect"
	"time"
)

// barIncrementer simplifies incrementing bar.
type barIncrementer struct {
	// bar is the bar to increment.
	bar *mpb.Bar
	// start shall be the work start time.
	start time.Time
}

// inc increments bi.bar by i.
func (bi *barIncrementer) inc(i int) {
	prev := bi.start
	now := time.Now()
	bi.start = now

	bi.bar.IncrBy(i)
	bi.bar.DecoratorEwmaUpdate(now.Sub(prev))
}

const bulk = 10000

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

// sliceIdoHistory performs query with args+map[string]interface{}{"checkpoint": checkpoint, "bulk": bulk} on snapshot
// and passes the results to onRows (a func([]rowType)interface{})
// until either an empty result set or onRows() returns nil.
// Rationale: split the likely large result set of a query by adding a WHERE condition and a LIMIT,
// both with :named placeholders (:checkpoint, :bulk).
// checkpoint is the initial value for the WHERE condition, onRows() returns follow-up ones.
// (On non-recoverable errors the whole program exits.)
// TODO: make onRows generic[T] one nice day
func sliceIdoHistory(
	snapshot *sqlx.Tx, query string, args map[string]interface{}, checkpoint interface{},
	rowType reflect.Type, onRows func(idoRows interface{}) (checkpoint interface{}),
) {
	vNewRows := reflect.New(reflect.SliceOf(rowType))
	rowsPtr := vNewRows.Interface()
	vRows := vNewRows.Elem()

	if args == nil {
		args = map[string]interface{}{}
	}

	args["checkpoint"] = checkpoint
	args["bulk"] = bulk

	for {
		// TODO: use Tx#SelectNamed() one nice day (https://github.com/jmoiron/sqlx/issues/779)
		stmt, err := snapshot.PrepareNamed(query)
		if err != nil {
			log.With("query", query).Fatalf("%+v", errors.Wrap(err, "can't prepare query"))
		}

		if err := stmt.Select(rowsPtr, args); err != nil {
			log.With("query", query).Fatalf("%+v", errors.Wrap(err, "can't perform query"))
		}

		_ = stmt.Close()

		if vRows.Len() < 1 {
			break
		}

		if checkpoint = onRows(vRows.Interface()); checkpoint == nil {
			break
		}

		vRows.Set(vRows.Slice(0, 0))
		args["checkpoint"] = checkpoint
	}
}

// historyType specifies a history data type.
type historyType struct {
	// name is a human-readable, but machine-friendly common name.
	name string
	// idoTable specifies the source table.
	idoTable string
	// idoIdColumn specifies idoTable's primary key.
	idoIdColumn string
	// cacheSchema specifies <name>.sqlite3's structure.
	cacheSchema []string
	// cacheFiller fills cache from snapshot.
	cacheFiller func(*historyType)
	// cacheLimitQuery rationale: see migrate().
	cacheLimitQuery string
	// migrationQuery SELECTs source data for actual migration.
	migrationQuery string
	// convertRows intention: see migrate().
	convertRows interface{}

	// cache represents <name>.sqlite3.
	cache *sqlx.DB
	// snapshot represents the data source.
	snapshot *sqlx.Tx
	// total summarizes the source data.
	total int64
	// done summarizes the migrated data.
	done int64
	// bar represents the current progress bar.
	bar *mpb.Bar
	// lastId is the last already migrated ID.
	lastId uint64
}

// setupBar (re-)initializes ht.bar.
func (ht *historyType) setupBar(progress *mpb.Progress) {
	ht.bar = progress.AddBar(
		ht.total,
		mpb.BarFillerClearOnComplete(),
		mpb.PrependDecorators(
			decor.Name(ht.name, decor.WC{W: len(ht.name) + 1, C: decor.DidentRight}),
			decor.Percentage(decor.WC{W: 5}),
		),
		mpb.AppendDecorators(decor.EwmaETA(decor.ET_STYLE_GO, 6000000, decor.WC{W: 4})),
	)
}

type historyTypes [6]historyType

// forEach performs f per *ht in parallel.
func (ht *historyTypes) forEach(f func(*historyType)) {
	eg, _ := errgroup.WithContext(context.Background())
	for i := range *ht {
		i := i
		eg.Go(func() error {
			f(&(*ht)[i])
			return nil
		})
	}

	_ = eg.Wait()
}

var types = historyTypes{
	{
		name:        "acknowledgement",
		idoTable:    "icinga_acknowledgements",
		idoIdColumn: "acknowledgement_id",
		cacheSchema: eventTimeCacheSchema,
		cacheFiller: func(ht *historyType) {
			buildEventTimeCache(ht, []string{
				"xh.acknowledgement_id id", "UNIX_TIMESTAMP(xh.entry_time) event_time",
				"xh.entry_time_usec event_time_usec", "xh.acknowledgement_type event_is_start", "xh.object_id",
			})
		},
		migrationQuery: acknowledgementMigrationQuery,
		convertRows:    convertAcknowledgementRows,
	},
	{
		name:           "comment",
		idoTable:       "icinga_commenthistory",
		idoIdColumn:    "commenthistory_id",
		migrationQuery: commentMigrationQuery,
		convertRows:    convertCommentRows,
	},
	{
		name:           "downtime",
		idoTable:       "icinga_downtimehistory",
		idoIdColumn:    "downtimehistory_id",
		migrationQuery: downtimeMigrationQuery,
		convertRows:    convertDowntimeRows,
	},
	{
		name:        "flapping",
		idoTable:    "icinga_flappinghistory",
		idoIdColumn: "flappinghistory_id",
		cacheSchema: eventTimeCacheSchema,
		cacheFiller: func(ht *historyType) {
			buildEventTimeCache(ht, []string{
				"xh.flappinghistory_id id", "UNIX_TIMESTAMP(xh.event_time) event_time",
				"xh.event_time_usec", "xh.event_type-1000 event_is_start", "xh.object_id",
			})
		},
		migrationQuery: flappingMigrationQuery,
		convertRows:    convertFlappingRows,
	},
	{
		name:        "notification",
		idoTable:    "icinga_notifications",
		idoIdColumn: "notification_id",
		cacheSchema: previousHardStateCacheSchema,
		cacheFiller: func(ht *historyType) {
			buildPreviousHardStateCache(ht, []string{
				"xh.notification_id id", "xh.object_id", "xh.state last_hard_state",
			})
		},
		cacheLimitQuery: "SELECT MAX(history_id) FROM previous_hard_state",
		migrationQuery:  notificationMigrationQuery,
		convertRows:     convertNotificationRows,
	},
	{
		name:        "state",
		idoTable:    "icinga_statehistory",
		idoIdColumn: "statehistory_id",
		cacheSchema: previousHardStateCacheSchema,
		cacheFiller: func(ht *historyType) {
			buildPreviousHardStateCache(ht, []string{"xh.statehistory_id id", "xh.object_id", "xh.last_hard_state"})
		},
		cacheLimitQuery: "SELECT MAX(history_id) FROM previous_hard_state",
		migrationQuery:  stateMigrationQuery,
		convertRows:     convertStateRows,
	},
}
