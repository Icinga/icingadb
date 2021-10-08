package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"github.com/google/uuid"
	"github.com/icinga/icingadb/pkg/icingadb/objectpacker"
	icingadbTypes "github.com/icinga/icingadb/pkg/types"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/vbauerster/mpb/v6"
	"github.com/vbauerster/mpb/v6/decor"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"reflect"
	"sync"
	"time"
)

// ProgressRow contains all IDO info needed to assemble an Icinga DB ID.
type ProgressRow struct {
	// Id is the IDO table ID.
	Id uint64
	// Name is the name of the affected comment or downtime.
	Name string
}

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

// mkDeterministicUuid returns a formally random UUID (v4) as follows: 11111122-3300-4455-4455-555555555555
//
// 0: zeroed
// 1: "IDO" (where the data identified by the new UUID is from)
// 2: the history table the new UUID is for, e.g. "s" for state_history
// 3: "h" (for "history")
// 4: the new UUID's formal version (unused bits zeroed)
// 5: the ID of the row the new UUID is for in the IDO (big endian)
//
// Rationale: be able to pre-calculate the IDs to figure out which have already been migrated.
func mkDeterministicUuid(table byte, rowId uint64) icingadbTypes.UUID {
	uid := uuidTemplate
	uid[3] = table

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.BigEndian, rowId); err != nil {
		panic(err)
	}

	bEId := buf.Bytes()
	uid[7] = bEId[0]
	copy(uid[9:], bEId[1:])

	return icingadbTypes.UUID{UUID: uid}
}

// uuidTemplate is for mkDeterministicUuid() to save a few CPU cycles.
var uuidTemplate = func() uuid.UUID {
	buf := &bytes.Buffer{}
	buf.Write(uuid.Nil[:])

	uid, err := uuid.NewRandomFromReader(buf)
	if err != nil {
		panic(err)
	}

	copy(uid[:], "IDO h")
	return uid
}()

// randomUuid generates a new UUIDv4. Saves getrandom(2) syscalls via bufio.Reader-s.
// (On non-recoverable errors the whole program exits.)
func randomUuid() icingadbTypes.UUID {
	var rander *bufio.Reader

	// Get random available rander.
	massRanders.Lock()
	for r := range massRanders.pool {
		rander = r
		delete(massRanders.pool, r)
		break
	}
	massRanders.Unlock()

	// Fall back to new one.
	if rander == nil {
		rander = bufio.NewReader(rand.Reader)
	}

	id, err := uuid.NewRandomFromReader(rander)
	if err != nil {
		log.Fatalf("%+v", errors.Wrap(err, "can't generate random UUID"))
	}

	// Make it available for the next call.
	massRanders.Lock()
	massRanders.pool[rander] = struct{}{}
	massRanders.Unlock()

	return icingadbTypes.UUID{UUID: id}
}

// massRanders are randomUuid()'s storage.
var massRanders = struct {
	sync.Mutex
	pool map[*bufio.Reader]struct{}
}{
	sync.Mutex{},
	map[*bufio.Reader]struct{}{},
}

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

// sliceIdoHistory performs query with args+[]interface{}{checkpoint,bulk} on snapshot and passes the results
// to onRows (a func([]T)interface{}) until either an empty result set or onRows() returns nil.
// Rationale: split the likely large result set of a query by adding a WHERE condition and a LIMIT,
// both with ? placeholders. Due to this function's internals they have to be the last two placeholders.
// checkpoint is the initial value for the WHERE condition, onRows() returns follow-up ones.
// (On non-recoverable errors the whole program exits.)
func sliceIdoHistory(snapshot *sqlx.Tx, query string, args []interface{}, checkpoint, onRows interface{}) {
	vOnRows := reflect.ValueOf(onRows) // TODO: make onRows generic[T] one nice day

	tRows := vOnRows.Type(). // func(rows []T) (checkpoint interface{})
					In(0) // []T

	vNewRows := reflect.New(tRows)
	rowsPtr := vNewRows.Interface()
	vRows := vNewRows.Elem()
	onRowsArgs := [1]reflect.Value{vRows}
	vZeroRows := reflect.Zero(tRows)
	args = append(append([]interface{}(nil), args...), checkpoint, bulk)

	for {
		if err := snapshot.Select(rowsPtr, query, args...); err != nil {
			log.With("query", query).Fatalf("%+v", errors.Wrap(err, "can't perform query"))
		}

		if vRows.Len() < 1 {
			break
		}

		if checkpoint = vOnRows.Call(onRowsArgs[:])[0].Interface(); checkpoint == nil {
			break
		}

		vRows.Set(vZeroRows)
		args[len(args)-2] = checkpoint
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
	// idoColumns specifies idoTable's columns in addition to idoIdColumn computeProgress() needs.
	idoColumns []string
	// idbTable specifies the destination table computeProgress() compares the source data with.
	idbTable string
	// idoIdColumn specifies idbTable's primary key.
	idbIdColumn string
	// convertId converts the IDO row and the Icinga 2 env name to a value suitable for idbIdColumn.
	convertId func(row ProgressRow, env string) []byte
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
		idbTable:    "history",
		idbIdColumn: "id",
		convertId:   func(row ProgressRow, _ string) []byte { u := mkDeterministicUuid('a', row.Id); return u.UUID[:] },
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
		idoColumns:     []string{"name"},
		idbTable:       "comment_history",
		idbIdColumn:    "comment_id",
		convertId:      func(row ProgressRow, env string) []byte { return calcObjectId(env, row.Name) },
		migrationQuery: commentMigrationQuery,
		convertRows:    convertCommentRows,
	},
	{
		name:           "downtime",
		idoTable:       "icinga_downtimehistory",
		idoIdColumn:    "downtimehistory_id",
		idoColumns:     []string{"name"},
		idbTable:       "downtime_history",
		idbIdColumn:    "downtime_id",
		convertId:      func(row ProgressRow, env string) []byte { return calcObjectId(env, row.Name) },
		migrationQuery: downtimeMigrationQuery,
		convertRows:    convertDowntimeRows,
	},
	{
		name:        "flapping",
		idoTable:    "icinga_flappinghistory",
		idoIdColumn: "flappinghistory_id",
		idbTable:    "history",
		idbIdColumn: "id",
		convertId:   func(row ProgressRow, _ string) []byte { u := mkDeterministicUuid('f', row.Id); return u.UUID[:] },
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
		idbTable:    "notification_history",
		idbIdColumn: "id",
		convertId:   func(row ProgressRow, _ string) []byte { u := mkDeterministicUuid('n', row.Id); return u.UUID[:] },
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
		idbTable:    "state_history",
		idbIdColumn: "id",
		convertId:   func(row ProgressRow, _ string) []byte { u := mkDeterministicUuid('s', row.Id); return u.UUID[:] },
		cacheSchema: previousHardStateCacheSchema,
		cacheFiller: func(ht *historyType) {
			buildPreviousHardStateCache(ht, []string{"xh.statehistory_id id", "xh.object_id", "xh.last_hard_state"})
		},
		cacheLimitQuery: "SELECT MAX(history_id) FROM previous_hard_state",
		migrationQuery:  stateMigrationQuery,
		convertRows:     convertStateRows,
	},
}
