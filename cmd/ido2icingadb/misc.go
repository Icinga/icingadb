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

type ProgressRow struct {
	Id   uint64
	Name string
}

type convertedId struct {
	ido uint64
	idb []byte
}

type barIncrementer struct {
	bar   *mpb.Bar
	start time.Time
}

func (bi *barIncrementer) inc(i int) {
	prev := bi.start
	now := time.Now()
	bi.start = now

	bi.bar.IncrBy(i)
	bi.bar.DecoratorEwmaUpdate(now.Sub(prev))
}

const bulk = 10000

var log = func() *zap.SugaredLogger {
	logger, err := zap.NewDevelopmentConfig().Build()
	if err != nil {
		panic(err)
	}

	return logger.Sugar()
}()

var objectTypes = map[uint8]string{1: "host", 2: "service"}

// mkDeterministicUuid returns a formally random UUID (v4) as follows: 11111122-3300-4455-4455-555555555555
//
// 0: zeroed
// 1: "IDO" (where the data identified by the new UUID is from)
// 2: the history table the new UUID is for, e.g. "s" for state_history
// 3: "h" (for "history")
// 4: the new UUID's formal version (unused bits zeroed)
// 5: the ID of the row the new UUID is for in the IDO (big endian)
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

// uuidTemplate is for mkDeterministicUuid.
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

// randomUuid generates a new UUIDv4.
func randomUuid() icingadbTypes.UUID {
	var rander *bufio.Reader

	massRanders.Lock()
	for r := range massRanders.pool {
		rander = r
		delete(massRanders.pool, r)
		break
	}
	massRanders.Unlock()

	if rander == nil {
		rander = bufio.NewReader(rand.Reader)
	}

	id, err := uuid.NewRandomFromReader(rander)
	if err != nil {
		log.Fatalf("%+v", errors.Wrap(err, "can't generate random UUID"))
	}

	massRanders.Lock()
	massRanders.pool[rander] = struct{}{}
	massRanders.Unlock()

	return icingadbTypes.UUID{UUID: id}
}

var massRanders = struct {
	sync.Mutex
	pool map[*bufio.Reader]struct{}
}{
	sync.Mutex{},
	map[*bufio.Reader]struct{}{},
}

// hashAny combines PackAny and SHA1 hashing.
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

type historyType struct {
	name            string
	idoTable        string
	idoIdColumn     string
	idoColumns      []string
	idbTable        string
	idbIdColumn     string
	convertId       func(row ProgressRow, env string) []byte
	cacheSchema     []string
	cacheFiller     func(*historyType)
	cacheLimitQuery string
	migrationQuery  string
	convertRows     interface{}

	cache    *sqlx.DB
	snapshot *sqlx.Tx
	total    int64
	bar      *mpb.Bar
	lastId   uint64
}

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
		"acknowledgement",
		"icinga_acknowledgements",
		"acknowledgement_id",
		nil,
		"history",
		"id",
		func(row ProgressRow, _ string) []byte { u := mkDeterministicUuid('a', row.Id); return u.UUID[:] },
		eventTimeCacheSchema,
		func(ht *historyType) {
			buildEventTimeCache(ht, []string{
				"xh.acknowledgement_id id", "UNIX_TIMESTAMP(xh.entry_time) event_time",
				"xh.entry_time_usec event_time_usec", "xh.acknowledgement_type event_is_start", "xh.object_id",
			})
		},
		"",
		acknowledgementMigrationQuery,
		convertAcknowledgementRows,
		nil, nil, 0, nil, 0,
	},
	{
		"comment",
		"icinga_commenthistory",
		"commenthistory_id",
		[]string{"name"},
		"comment_history",
		"comment_id",
		func(row ProgressRow, env string) []byte { return calcObjectId(env, row.Name) },
		nil, nil, "",
		commentMigrationQuery,
		convertCommentRows,
		nil, nil, 0, nil, 0,
	},
	{
		"downtime",
		"icinga_downtimehistory",
		"downtimehistory_id",
		[]string{"name"},
		"downtime_history",
		"downtime_id",
		func(row ProgressRow, env string) []byte { return calcObjectId(env, row.Name) },
		nil, nil, "",
		downtimeMigrationQuery,
		convertDowntimeRows,
		nil, nil, 0, nil, 0,
	},
	{
		"flapping",
		"icinga_flappinghistory",
		"flappinghistory_id",
		nil,
		"history",
		"id",
		func(row ProgressRow, _ string) []byte { u := mkDeterministicUuid('f', row.Id); return u.UUID[:] },
		eventTimeCacheSchema,
		func(ht *historyType) {
			buildEventTimeCache(ht, []string{
				"xh.flappinghistory_id id", "UNIX_TIMESTAMP(xh.event_time) event_time",
				"xh.event_time_usec", "xh.event_type-1000 event_is_start", "xh.object_id",
			})
		},
		"",
		flappingMigrationQuery,
		convertFlappingRows,
		nil, nil, 0, nil, 0,
	},
	{
		"notification",
		"icinga_notifications",
		"notification_id",
		nil,
		"notification_history",
		"id",
		func(row ProgressRow, _ string) []byte { u := mkDeterministicUuid('n', row.Id); return u.UUID[:] },
		previousHardStateCacheSchema,
		func(ht *historyType) {
			buildPreviousHardStateCache(ht, []string{
				"xh.notification_id id", "xh.object_id", "xh.state last_hard_state",
			})
		},
		"SELECT MAX(history_id) FROM previous_hard_state",
		notificationMigrationQuery,
		convertNotificationRows,
		nil, nil, 0, nil, 0,
	},
	{
		"state",
		"icinga_statehistory",
		"statehistory_id",
		nil,
		"state_history",
		"id",
		func(row ProgressRow, _ string) []byte { u := mkDeterministicUuid('s', row.Id); return u.UUID[:] },
		previousHardStateCacheSchema,
		func(ht *historyType) {
			buildPreviousHardStateCache(ht, []string{"xh.statehistory_id id", "xh.object_id", "xh.last_hard_state"})
		},
		"SELECT MAX(history_id) FROM previous_hard_state",
		stateMigrationQuery,
		convertStateRows,
		nil, nil, 0, nil, 0,
	},
}