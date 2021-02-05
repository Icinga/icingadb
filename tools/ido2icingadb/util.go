package main

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/cheggaaa/pb/v3"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"sync"
	"syscall"
)

// historyTable represents Icinga DB history tables.
type historyTable byte

const (
	ackHistory          historyTable = 'a'
	flappingHistory     historyTable = 'f'
	notificationHistory historyTable = 'n'
	stateHistory        historyTable = 's'
)

var bools = map[uint8]string{0: "n", 1: "y"}
var objectTypes = map[uint8]string{1: "host", 2: "service"}
var commentTypes = map[uint8]string{1: "comment", 4: "ack"}
var stateTypes = map[uint8]string{0: "soft", 1: "hard"}

var notificationTypes = map[uint8]string{
	5: "downtime_start", 6: "downtime_end", 7: "downtime_removed",
	8: "custom", 1: "acknowledgement", 2: "flapping_start", 3: "flapping_end",
}

// bulkInsert represents several rows to be inserted via stmt.
type bulkInsert struct {
	stmt string
	rows [][]interface{}
}

// stringValue allows to differ a string not passed via the CLI and an empty string passed via the CLI
// w/o polluting the usage instructions.
type stringValue struct {
	// value is the string passed via the CLI if any.
	value string
	// isSet tells whether the string was passed.
	isSet bool
}

var _ flag.Value = (*stringValue)(nil)

// String implements flag.Value.
func (sv *stringValue) String() string {
	return sv.value
}

// Set implements flag.Value.
func (sv *stringValue) Set(s string) error {
	sv.value = s
	sv.isSet = true
	return nil
}

type progress struct {
	total, done int64
}

// multiTaskBar lets multiple workers report their progress to a single progress bar.
type multiTaskBar struct {
	// progress contains the progress per worker.
	progress chan progress
	// bar indicates the overall progress.
	bar *pb.ProgressBar
	// start indicates that bar is ready.
	start chan struct{}
	// wg indicates that the workers are done.
	wg sync.WaitGroup
}

// runMaster coordinates everything and waits until the workers are done.
func (mtb *multiTaskBar) runMaster() {
	var progress progress
	for i := cap(mtb.progress); i > 0; i-- {
		p := <-mtb.progress

		progress.total += p.total
		progress.done += p.done
	}

	mtb.bar = pb.New64(progress.total).SetCurrent(progress.done).Start()
	close(mtb.start)

	mtb.wg.Wait()
	mtb.bar.Finish()
}

// startWorker shall be called once per worker with their individual progress.
func (mtb *multiTaskBar) startWorker(total, done int64) *pb.ProgressBar {
	mtb.progress <- progress{total, done}
	<-mtb.start
	return mtb.bar
}

// stopWorker shall be called once per worker once done.
func (mtb *multiTaskBar) stopWorker() {
	mtb.wg.Done()
}

// newMultiTaskBar creates a new multiTaskBar suitable for workers workers.
func newMultiTaskBar(workers int) *multiTaskBar {
	mtb := &multiTaskBar{
		progress: make(chan progress, workers),
		start:    make(chan struct{}),
	}

	mtb.wg.Add(workers)
	return mtb
}

// assert logs message with fields and err and terminates the program if err is not nil.
func assert(err error, message string, fields log.Fields) {
	if err != nil {
		{
			var retry bool
			switch tErr := err.(type) {
			case *mysql.MySQLError:
				retry = tErr.Number == 1205
			default:
				// Likely while streaming a large result of a MySQL query the connection suddenly broke.
				retry = err == mysql.ErrInvalidConn
			}

			if retry {
				log.WithFields(fields).WithFields(log.Fields{"error": err.Error()}).Error(message)

				// Luckily we can just "travel back in time" via exec(3), but preserve our progress.
				log.Warn("Re-trying")
				assert(syscall.Exec(os.Args[0], os.Args, os.Environ()), "Couldn't re-exec(3) program", nil)
			}
		}

		log.WithFields(fields).WithFields(log.Fields{"error": err.Error()}).Fatal(message)
	}
}

// flush runs bulks on icingaDb in a single transaction.
func flush(bulks ...bulkInsert) {
	tx := icingaDb.begin(sql.LevelReadCommitted, false)

	for _, b := range bulks {
		if len(b.rows) < 1 {
			continue
		}

		stmt, errPp := tx.tx.Prepare(b.stmt)
		assert(errPp, "Couldn't prepare SQL statement", log.Fields{"backend": icingaDb.whichOne, "statement": b.stmt})

		for _, r := range b.rows {
			_, errEx := stmt.Exec(r...)
			assert(
				errEx, "Couldn't execute prepared SQL statement",
				log.Fields{"backend": icingaDb.whichOne, "statement": b.stmt, "args": r},
			)
		}

		assert(
			stmt.Close(), "Couldn't close prepared SQL statement",
			log.Fields{"backend": icingaDb.whichOne, "statement": b.stmt},
		)
	}

	tx.commit()
}

// getProgress bisects the range of idoIdColumn in idoTable as UUIDs in icingadbTable's icingadbIdColumn using idoTx
// and returns the current progress and an idoIdColumn value to start/continue sync with.
func getProgress(
	idoTx tx, idoTable, idoIdColumn, icingadbTable, icingadbIdColumn string,
	mkIcingadbId func(idoId uint64) (icingadbId interface{}),
) (
	total, done, lastSyncedId int64,
) {
	var left, right sql.NullInt64

	idoTx.query(
		fmt.Sprintf("SELECT MIN(%s), MAX(%s) FROM %s", idoIdColumn, idoIdColumn, idoTable),
		nil,
		func(row struct{ Min, Max sql.NullInt64 }) {
			left = row.Min
			right = row.Max
		},
	)

	if !left.Valid {
		return
	}

	left.Int64 -= 1
	query := fmt.Sprintf("SELECT 1 FROM %s WHERE %s=?", icingadbTable, icingadbIdColumn)
	total = right.Int64 - left.Int64
	firstLeft := left.Int64

	for {
		lastSyncedId = right.Int64 - (right.Int64-left.Int64)/2

		has := false
		icingaDb.query(
			query,
			[]interface{}{mkIcingadbId(uint64(lastSyncedId))},
			func(struct{ One uint8 }) { has = true },
		)

		if has {
			left.Int64 = lastSyncedId
		} else {
			lastSyncedId--
			right.Int64 = lastSyncedId
		}

		if left.Int64 == right.Int64 {
			done = left.Int64 - firstLeft
			return
		}
	}
}

// uuidTemplate is for mkDeterministicUuid.
var uuidTemplate = func() uuid.UUID {
	buf := &bytes.Buffer{}
	buf.Write(uuid.Nil[:])

	uid, errNR := uuid.NewRandomFromReader(buf)
	if errNR != nil {
		panic(errNR)
	}

	copy(uid[:], "IDO h")

	return uid
}()

// mkDeterministicUuid returns a formally random UUID (v4) as follows: 11111122-3300-4455-4455-555555555555
//
// 0: zeroed
// 1: "IDO" (where the data identified by the new UUID is from)
// 2: the history table the new UUID is for, e.g. "s" for state_history
// 3: "h" (for "history")
// 4: the new UUID's formal version (unused bits zeroed)
// 5: the ID of the row the new UUID is for in the IDO (big endian)
func mkDeterministicUuid(table historyTable, rowId uint64) []byte {
	uid := uuidTemplate
	uid[3] = byte(table)

	buf := &bytes.Buffer{}
	if errWB := binary.Write(buf, binary.BigEndian, rowId); errWB != nil {
		panic(errWB)
	}

	bEId := buf.Bytes()
	uid[7] = bEId[0]
	copy(uid[9:], bEId[1:])

	return uid[:]
}

// mkRandomUuid generates a new UUIDv4.
func mkRandomUuid(rander io.Reader) []byte {
	id, errNR := uuid.NewRandomFromReader(rander)
	assert(errNR, "Couldn't generate random UUID", nil)
	return id[:]
}

// convertTime converts *nix timestamps from the IDO for Icinga DB.
func convertTime(ts int64, tsUs uint32) uint64 {
	return uint64(ts)*1000 + uint64(tsUs)/1000
}
