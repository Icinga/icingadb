package main

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"fmt"
	"github.com/cheggaaa/pb/v3"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// historyTable represents Icinga DB history tables.
type historyTable byte

const (
	stateHistory historyTable = 's'
)

// bulkInsert represents several rows to be inserted via stmt.
type bulkInsert struct {
	stmt string
	rows [][]interface{}
}

var objectTypes = map[uint8]string{1: "host", 2: "service"}
var stateTypes = map[uint8]string{0: "soft", 1: "hard"}

// syncStates migrates IDO's icinga_statehistory table to Icinga DB's state_history table using the -cache FILE.
func syncStates() {
	// Icinga DB's state_history#previous_hard_state would need a subquery.
	// That make the IDO reading even slower than the Icinga DB writing.
	// Therefore: Stream IDO's icinga_statehistory once, compute state_history#previous_hard_state
	// and cache it into an SQLite database. Then steam from that database and the IDO.

	dbc, errOp := sql.Open("sqlite3", "file:"+cache.value)
	assert(errOp, "Couldn't open SQLite3 database", log.Fields{"file": cache.value})
	defer dbc.Close()

	cach := &database{
		whichOne: "cache",
		conn:     dbc,
	}

	// Icinga DB's state_history#previous_hard_state per IDO's icinga_statehistory#statehistory_id.
	cach.exec(`CREATE TABLE IF NOT EXISTS previous_hard_state (
	statehistory_id INT PRIMARY KEY,
	previous_hard_state INT NOT NULL
)`)

	// Helper table, the current last_hard_state per icinga_statehistory#object_id.
	cach.exec(`CREATE TABLE IF NOT EXISTS next_hard_state (
	object_id INT PRIMARY KEY,
	next_hard_state INT NOT NULL
)`)

	// Helper table for stashing icinga_statehistory#statehistory_id until last_hard_state changes.
	cach.exec(`CREATE TABLE IF NOT EXISTS next_ids (
	object_id INT NOT NULL,
	statehistory_id INT NOT NULL
)`)

	cach.exec("CREATE INDEX IF NOT EXISTS next_ids_object_id ON next_ids(object_id)")
	cach.exec("CREATE INDEX IF NOT EXISTS next_ids_statehistory_id ON next_ids(statehistory_id)")

	snapshot := ido.begin(sql.LevelRepeatableRead, true)
	defer snapshot.commit()

	total, done, lsi := getProgress(snapshot, stateHistory, "icinga_statehistory", "statehistory_id", "state_history")
	var limit []struct{ StatehistoryId sql.NullInt64 }

	{
		bar := pb.StartNew(int(total))
		tx := cach.begin(sql.LevelSerializable, false)
		var checkpoint int64

		var niCMShi []struct {
			Count          uint64
			StatehistoryId sql.NullInt64
		}
		tx.fetchAll(&niCMShi, "SELECT COUNT(*), MIN(statehistory_id) FROM next_ids")

		var phsC []struct{ Count uint64 }
		tx.fetchAll(&phsC, "SELECT COUNT(*) FROM previous_hard_state")

		if niCMShi[0].StatehistoryId.Valid {
			checkpoint = niCMShi[0].StatehistoryId.Int64
		} else {
			// next_ids contains the most recently processed IDs and is only empty if...
			if phsC[0].Count == 0 {
				// ... we didn't actually start, yet...
				checkpoint = 9223372036854775807
			} else {
				// ... or we already finished.
				checkpoint = 0
			}
		}

		bar.Add(int(phsC[0].Count + niCMShi[0].Count))
		inTx := 0

		// We continue where we finished before. As we build the cache in reverse chronological order:
		// 1. If the history grows between two migration trials, we won't migrate the difference. Workarounds:
		//    a. Start migration after Icinga DB is up and running.
		//    b. Remove the -cache FILE before the next migration trial.
		// 2. If the history gets cleaned up between two migration trials,
		//    the difference either just doesn't appear in the cache or - if already there - will be ignored later.

		snapshot.query(
			"SELECT statehistory_id, object_id, last_hard_state FROM icinga_statehistory "+
				"WHERE statehistory_id < ? ORDER BY statehistory_id DESC",
			[]interface{}{checkpoint},
			func(row struct {
				StatehistoryId uint64
				ObjectId       uint64
				LastHardState  uint8
			}) {
				var nhs []struct{ NextHardState uint8 }
				tx.fetchAll(&nhs, "SELECT next_hard_state FROM next_hard_state WHERE object_id=?", row.ObjectId)

				if len(nhs) < 1 {
					tx.exec(
						"INSERT INTO next_hard_state(object_id, next_hard_state) VALUES (?, ?)",
						row.ObjectId, row.LastHardState,
					)

					tx.exec(
						"INSERT INTO next_ids(statehistory_id, object_id) VALUES (?, ?)",
						row.StatehistoryId, row.ObjectId,
					)
				} else if row.LastHardState == nhs[0].NextHardState {
					tx.exec(
						"INSERT INTO next_ids(statehistory_id, object_id) VALUES (?, ?)",
						row.StatehistoryId, row.ObjectId,
					)
				} else {
					tx.exec(
						"INSERT INTO previous_hard_state(statehistory_id, previous_hard_state) "+
							"SELECT statehistory_id, ? FROM next_ids WHERE object_id=?",
						row.LastHardState, row.ObjectId,
					)

					tx.exec("DELETE FROM next_hard_state WHERE object_id=?", row.ObjectId)
					tx.exec("DELETE FROM next_ids WHERE object_id=?", row.ObjectId)

					tx.exec(
						"INSERT INTO next_hard_state(object_id, next_hard_state) VALUES (?, ?)",
						row.ObjectId, row.LastHardState,
					)

					tx.exec(
						"INSERT INTO next_ids(statehistory_id, object_id) VALUES (?, ?)",
						row.StatehistoryId, row.ObjectId,
					)
				}

				inTx++
				if inTx == *bulk {
					tx.commit()
					tx = cach.begin(sql.LevelSerializable, false)
					inTx = 0
				}

				bar.Increment()
			},
		)

		tx.exec(
			"INSERT INTO previous_hard_state(statehistory_id, previous_hard_state) " +
				"SELECT statehistory_id, 99 FROM next_ids",
		)

		tx.exec("DELETE FROM next_hard_state")
		tx.exec("DELETE FROM next_ids")

		tx.fetchAll(&limit, "SELECT MAX(statehistory_id) FROM previous_hard_state")

		tx.commit()
		bar.Finish()
	}

	previousHardStates := make(chan uint8, 64)

	// Stream concurrently from two databases. Possible due to WHERE and ORDER BY.
	go cach.query(
		"SELECT previous_hard_state FROM previous_hard_state WHERE statehistory_id >= ? ORDER BY statehistory_id",
		[]interface{}{lsi},
		func(row struct{ PreviousHardState uint8 }) {
			previousHardStates <- row.PreviousHardState
		},
	)

	bar := pb.StartNew(int(total))
	bar.Add(int(done))

	sh := bulkInsert{
		stmt: "REPLACE INTO state_history(id, environment_id, endpoint_id, object_type, host_id, " +
			"service_id, event_time, state_type, soft_state, hard_state, previous_soft_state, " +
			"previous_hard_state, attempt, output, long_output, max_check_attempts, check_source) " +
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
	}

	h := bulkInsert{
		stmt: "REPLACE INTO history(id, environment_id, endpoint_id, object_type, host_id, service_id, " +
			"state_history_id, event_type, event_time) VALUES (?, ?, ?, ?, ?, ?, ?, 'state_change', ?)",
	}

	snapshot.query(
		"SELECT sh.statehistory_id, UNIX_TIMESTAMP(sh.state_time), sh.state_time_usec, "+
			"sh.state_change, sh.state, sh.state_type, sh.current_check_attempt, sh.max_check_attempts, "+
			"sh.last_state, sh.last_hard_state, sh.output, sh.long_output, sh.check_source, "+
			"o.objecttype_id, o.name1, IFNULL(o.name2, '') "+
			"FROM icinga_statehistory sh "+
			"INNER JOIN icinga_objects o ON o.object_id=sh.object_id "+
			"WHERE sh.statehistory_id BETWEEN ? AND ? "+
			"ORDER BY sh.statehistory_id",
		[]interface{}{lsi, limit[0].StatehistoryId},
		func(row struct {
			StatehistoryId uint64
			StateTime      int64
			StateTimeUsec  uint32

			StateChange         uint8
			State               uint8
			StateType           uint8
			CurrentCheckAttempt uint16
			MaxCheckAttempts    uint16

			LastState     uint8
			LastHardState uint8
			Output        string
			LongOutput    string
			CheckSource   string

			ObjecttypeId uint8
			Name1        string
			Name2        string
		}) {
			id := mkDeterministicUuid(stateHistory, row.StatehistoryId)
			typ := objectTypes[row.ObjecttypeId]
			hostId := calcObjectId(row.Name1)
			serviceId := calcServiceId(row.Name1, row.Name2)
			ts := convertTime(row.StateTime, row.StateTimeUsec)

			sh.rows = append(sh.rows, []interface{}{
				id, envId, endpointId, typ, hostId, serviceId, ts, stateTypes[row.StateType], row.State,
				row.LastHardState, row.LastState, <-previousHardStates, row.CurrentCheckAttempt,
				row.Output, row.LongOutput, row.MaxCheckAttempts, row.CheckSource,
			})

			h.rows = append(h.rows, []interface{}{id, envId, endpointId, typ, hostId, serviceId, id, ts})

			if len(sh.rows) == *bulk {
				flush(sh, h)

				sh.rows = nil
				h.rows = nil
			}

			bar.Increment()
		},
	)

	if len(sh.rows) > 0 {
		flush(sh, h)
	}

	bar.Finish()
}

// flush runs bulks on icingaDb in a single transaction.
func flush(bulks ...bulkInsert) {
	tx := icingaDb.begin(sql.LevelReadCommitted, false)

	for _, b := range bulks {
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

// getProgress bisects the range of idoIdColumn in idoTable as UUIDs in icingadbTable using idoTx
// and returns the current progress and an idoIdColumn value to start/continue sync with.
func getProgress(idoTx tx, table historyTable, idoTable, idoIdColumn, icingadbTable string) (
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

	query := fmt.Sprintf("SELECT 1 FROM %s WHERE id=?", icingadbTable)
	total = right.Int64 - left.Int64 + 1
	firstId := left.Int64

	for {
		if lastSyncedId = left.Int64 + (right.Int64-left.Int64)/2; lastSyncedId == left.Int64 {
			done = lastSyncedId - firstId
			return
		}

		has := false
		icingaDb.query(
			query,
			[]interface{}{mkDeterministicUuid(table, uint64(lastSyncedId))},
			func(struct{ One uint8 }) { has = true },
		)

		if has {
			left.Int64 = lastSyncedId
		} else {
			right.Int64 = lastSyncedId
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

// convertTime converts *nix timestamps from the IDO for Icinga DB.
func convertTime(ts int64, tsUs uint32) uint64 {
	return uint64(ts)*1000 + uint64(tsUs)/1000
}
