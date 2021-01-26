package main

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"fmt"
	"github.com/cheggaaa/pb/v3"
	"github.com/google/uuid"
)

// historyTable represents Icinga DB history tables.
type historyTable byte

const (
	stateHistory historyTable = 's'
)

var objectTypes = map[uint8]string{1: "host", 2: "service"}
var stateTypes = map[uint8]string{0: "soft", 1: "hard"}

func syncStates() {
	total, done, lsi := getProgress(stateHistory, "icinga_statehistory", "statehistory_id", "state_history")

	bar := pb.StartNew(int(total))
	bar.Add(int(done))

	ido.query(
		"SELECT sh.statehistory_id, UNIX_TIMESTAMP(sh.state_time), sh.state_time_usec, "+
			"sh.state_change, sh.state, sh.state_type, sh.current_check_attempt, sh.max_check_attempts, "+
			"sh.last_state, sh.last_hard_state, sh.output, sh.long_output, sh.check_source, "+
			"o.objecttype_id, o.name1, IFNULL(o.name2, ''), "+
			"IFNULL((SELECT sh2.last_hard_state "+
			"FROM icinga_statehistory sh2 "+
			"WHERE sh2.object_id=sh.object_id AND sh2.statehistory_id < sh.statehistory_id AND "+
			"sh2.last_hard_state<>sh.last_hard_state ORDER BY sh2.statehistory_id DESC LIMIT 1), 99) "+
			"FROM icinga_statehistory sh "+
			"INNER JOIN icinga_objects o ON o.object_id=sh.object_id "+
			"WHERE sh.statehistory_id >= ? "+
			"ORDER BY sh.statehistory_id",
		[]interface{}{lsi},
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

			PreviousHardState uint8
		}) {
			id := mkDeterministicUuid(stateHistory, row.StatehistoryId)
			typ := objectTypes[row.ObjecttypeId]
			hostId := calcObjectId(row.Name1)
			serviceId := calcServiceId(row.Name1, row.Name2)
			ts := convertTime(row.StateTime, row.StateTimeUsec)

			icingaDb.exec(
				"REPLACE INTO state_history(id, environment_id, endpoint_id, object_type, host_id, "+
					"service_id, event_time, state_type, soft_state, hard_state, previous_soft_state, "+
					"previous_hard_state, attempt, output, long_output, max_check_attempts, check_source) "+
					"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
				id, envId, endpointId, typ, hostId, serviceId, ts, stateTypes[row.StateType], row.State,
				row.LastHardState, row.LastState, row.PreviousHardState, row.CurrentCheckAttempt,
				row.Output, row.LongOutput, row.MaxCheckAttempts, row.CheckSource,
			)

			icingaDb.exec(
				"REPLACE INTO history(id, environment_id, endpoint_id, object_type, host_id, service_id, "+
					"state_history_id, event_type, event_time) VALUES (?, ?, ?, ?, ?, ?, ?, 'state_change', ?)",
				id, envId, endpointId, typ, hostId, serviceId, id, ts,
			)

			bar.Increment()
		},
	)

	bar.Finish()
}

// getProgress bisects the range of idoIdColumn in idoTable as UUIDs in icingadbTable
// and returns the current progress and an idoIdColumn value to start/continue sync with.
func getProgress(table historyTable, idoTable, idoIdColumn, icingadbTable string) (total, done, lastSyncedId int64) {
	var left, right sql.NullInt64

	ido.query(
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
