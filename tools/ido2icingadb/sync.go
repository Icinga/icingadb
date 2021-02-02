package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"io"
)

// historyTable represents Icinga DB history tables.
type historyTable byte

const (
	notificationHistory historyTable = 'n'
	stateHistory        historyTable = 's'
)

// bulkInsert represents several rows to be inserted via stmt.
type bulkInsert struct {
	stmt string
	rows [][]interface{}
}

var bools = map[uint8]string{0: "n", 1: "y"}
var objectTypes = map[uint8]string{1: "host", 2: "service"}
var stateTypes = map[uint8]string{0: "soft", 1: "hard"}

var notificationTypes = map[uint8]string{
	5: "downtime_start", 6: "downtime_end", 7: "downtime_removed",
	8: "custom", 1: "acknowledgement", 2: "flapping_start", 3: "flapping_end",
}

// syncDowntimes migrates IDO's icinga_downtimehistory table to Icinga DB's downtime_history table.
func syncDowntimes() {
	snapshot := ido.begin(sql.LevelRepeatableRead, true)
	defer snapshot.commit()

	total, done, ldi := getProgress(
		snapshot, "icinga_downtimehistory", "downtimehistory_id", "downtime_history", "downtime_id",
		func(idoId uint64) interface{} {
			var res []struct{ Name string }
			snapshot.fetchAll(&res, "SELECT name FROM icinga_downtimehistory WHERE downtimehistory_id=?", idoId)

			if len(res) < 1 {
				return make([]byte, 20)
			} else {
				return calcObjectId(res[0].Name)
			}
		},
	)

	bar := syncBar.startWorker(int(total))
	bar.Add(int(done))

	dh := bulkInsert{
		stmt: "REPLACE INTO downtime_history(downtime_id, environment_id, endpoint_id, triggered_by_id, object_type, " +
			"host_id, service_id, entry_time, author, comment, is_flexible, flexible_duration, " +
			"scheduled_start_time, scheduled_end_time, start_time, end_time, has_been_cancelled, trigger_time, " +
			"cancel_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
	}

	h := bulkInsert{
		stmt: "REPLACE INTO history(id, environment_id, endpoint_id, object_type, host_id, service_id, " +
			"downtime_history_id, event_type, event_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
	}

	{
		ch := make(chan struct {
			EntryTime   int64
			AuthorName  string
			CommentData string
			IsFixed     uint8
			Duration    int64

			ScheduledStartTime int64
			ScheduledEndTime   int64

			ActualStartTime     int64
			ActualStartTimeUsec uint32

			ActualEndTime     int64
			ActualEndTimeUsec uint32
			WasCancelled      uint8

			TriggerTime int64
			Name        string

			MonObjecttypeId uint8
			MonObjName1     string
			MonObjName2     string

			TriggeredBy string
		}, chSize)

		go streamQuery(
			snapshot,
			ch,
			"SELECT UNIX_TIMESTAMP(dh.entry_time), dh.author_name, dh.comment_data, dh.is_fixed, dh.duration, "+
				"UNIX_TIMESTAMP(dh.scheduled_start_time), UNIX_TIMESTAMP(dh.scheduled_end_time), "+
				"IFNULL(UNIX_TIMESTAMP(dh.actual_start_time), 0), dh.actual_start_time_usec, "+
				"IFNULL(UNIX_TIMESTAMP(dh.actual_end_time), 0), dh.actual_end_time_usec, dh.was_cancelled, "+
				"IFNULL(UNIX_TIMESTAMP(dh.trigger_time), 0), dh.name, "+
				"o.objecttype_id, o.name1, IFNULL(o.name2, ''), "+
				"IFNULL(sd.name, '') "+
				"FROM icinga_downtimehistory dh "+
				"INNER JOIN icinga_objects o ON o.object_id=dh.object_id "+
				"LEFT JOIN icinga_scheduleddowntime sd ON sd.scheduleddowntime_id=dh.triggered_by_id "+
				"WHERE dh.downtimehistory_id > ? "+
				"ORDER BY dh.downtimehistory_id",
			ldi,
		)

		massRander := bufio.NewReader(rand.Reader)

		for row := range ch {
			id := calcObjectId(row.Name)
			typ := objectTypes[row.MonObjecttypeId]
			hostId := calcObjectId(row.MonObjName1)
			serviceId := calcServiceId(row.MonObjName1, row.MonObjName2)
			start := convertTime(row.ActualStartTime, row.ActualStartTimeUsec)
			end := convertTime(row.ActualEndTime, row.ActualEndTimeUsec)

			var cancelTime uint64
			if row.WasCancelled != 0 {
				cancelTime = end
			}

			dh.rows = append(dh.rows, []interface{}{
				id, envId, endpointId, calcObjectId(row.TriggeredBy), typ, hostId, serviceId, row.EntryTime * 1000,
				row.AuthorName, row.CommentData, bools[1-row.IsFixed], row.Duration * 1000,
				row.ScheduledStartTime * 1000, row.ScheduledEndTime * 1000, start, end, bools[row.WasCancelled],
				row.TriggerTime * 1000, cancelTime,
			})

			h.rows = append(h.rows, []interface{}{
				mkRandomUuid(massRander), envId, endpointId, typ, hostId, serviceId, id, "downtime_start", start,
			})

			if end != 0 {
				h.rows = append(h.rows, []interface{}{
					mkRandomUuid(massRander), envId, endpointId, typ, hostId, serviceId, id, "downtime_end", end,
				})
			}

			if len(dh.rows) == *bulk {
				flush(dh, h)

				dh.rows = nil
				h.rows = nil
			}

			bar.Increment()
		}

		<-ch
	}

	if len(dh.rows) > 0 {
		flush(dh, h)
	}

	syncBar.stopWorker()
}

// syncNotifications migrates IDO's icinga_notifications table
// to Icinga DB's notification_history table using the -nh-cache FILE similar to syncStates.
func syncNotifications() {
	dbc, errOp := sql.Open("sqlite3", "file:"+nHcache.value)
	assert(errOp, "Couldn't open SQLite3 database", log.Fields{"file": nHcache.value})
	defer dbc.Close()

	cach := &database{
		whichOne: "-nh-cache",
		conn:     dbc,
	}

	cach.exec(`CREATE TABLE IF NOT EXISTS previous_hard_state (
	notification_id INT PRIMARY KEY,
	previous_hard_state INT NOT NULL
)`)

	cach.exec(`CREATE TABLE IF NOT EXISTS next_hard_state (
	object_id INT PRIMARY KEY,
	next_hard_state INT NOT NULL
)`)

	cach.exec(`CREATE TABLE IF NOT EXISTS next_ids (
	object_id INT NOT NULL,
	notification_id INT NOT NULL
)`)

	cach.exec("CREATE INDEX IF NOT EXISTS next_ids_object_id ON next_ids(object_id)")
	cach.exec("CREATE INDEX IF NOT EXISTS next_ids_notification_id ON next_ids(notification_id)")

	snapshot := ido.begin(sql.LevelRepeatableRead, true)
	defer snapshot.commit()

	total, done, lni := getProgress(
		snapshot, "icinga_notifications", "notification_id", "notification_history", "id",
		func(idoId uint64) interface{} { return mkDeterministicUuid(notificationHistory, idoId) },
	)

	var limit []struct{ NotificationId sql.NullInt64 }

	{
		bar := cacheBar.startWorker(int(total))
		tx := cach.begin(sql.LevelSerializable, false)
		var checkpoint int64

		var niCMNi []struct {
			Count          uint64
			NotificationId sql.NullInt64
		}
		tx.fetchAll(&niCMNi, "SELECT COUNT(*), MIN(notification_id) FROM next_ids")

		var phsC []struct{ Count uint64 }
		tx.fetchAll(&phsC, "SELECT COUNT(*) FROM previous_hard_state")

		if niCMNi[0].NotificationId.Valid {
			checkpoint = niCMNi[0].NotificationId.Int64
		} else {
			if phsC[0].Count == 0 {
				checkpoint = 9223372036854775807
			} else {
				checkpoint = 0
			}
		}

		bar.Add(int(phsC[0].Count + niCMNi[0].Count))
		inTx := 0

		{
			ch := make(chan struct {
				NotificationId uint64
				ObjectId       uint64
				State          uint8
			}, chSize)

			go streamQuery(
				snapshot,
				ch,
				"SELECT notification_id, object_id, state FROM icinga_notifications "+
					"WHERE notification_id < ? ORDER BY notification_id DESC",
				checkpoint,
			)

			for row := range ch {
				var nhs []struct{ NextHardState uint8 }
				tx.fetchAll(&nhs, "SELECT next_hard_state FROM next_hard_state WHERE object_id=?", row.ObjectId)

				if len(nhs) < 1 {
					tx.exec(
						"INSERT INTO next_hard_state(object_id, next_hard_state) VALUES (?, ?)",
						row.ObjectId, row.State,
					)

					tx.exec(
						"INSERT INTO next_ids(notification_id, object_id) VALUES (?, ?)",
						row.NotificationId, row.ObjectId,
					)
				} else if row.State == nhs[0].NextHardState {
					tx.exec(
						"INSERT INTO next_ids(notification_id, object_id) VALUES (?, ?)",
						row.NotificationId, row.ObjectId,
					)
				} else {
					tx.exec(
						"INSERT INTO previous_hard_state(notification_id, previous_hard_state) "+
							"SELECT notification_id, ? FROM next_ids WHERE object_id=?",
						row.State, row.ObjectId,
					)

					tx.exec("DELETE FROM next_hard_state WHERE object_id=?", row.ObjectId)
					tx.exec("DELETE FROM next_ids WHERE object_id=?", row.ObjectId)

					tx.exec(
						"INSERT INTO next_hard_state(object_id, next_hard_state) VALUES (?, ?)",
						row.ObjectId, row.State,
					)

					tx.exec(
						"INSERT INTO next_ids(notification_id, object_id) VALUES (?, ?)",
						row.NotificationId, row.ObjectId,
					)
				}

				inTx++
				if inTx == *bulk {
					tx.commit()
					tx = cach.begin(sql.LevelSerializable, false)
					inTx = 0
				}

				bar.Increment()
			}

			<-ch
		}

		tx.exec(
			"INSERT INTO previous_hard_state(notification_id, previous_hard_state) " +
				"SELECT notification_id, 99 FROM next_ids",
		)

		tx.exec("DELETE FROM next_hard_state")
		tx.exec("DELETE FROM next_ids")

		tx.fetchAll(&limit, "SELECT MAX(notification_id) FROM previous_hard_state")

		tx.commit()
		cach.exec("VACUUM")

		cacheBar.stopWorker()
	}

	bar := syncBar.startWorker(int(total))
	bar.Add(int(done))

	previousHardStates := make(chan struct{ PreviousHardState uint8 }, 64)

	go streamQuery(
		cach,
		previousHardStates,
		"SELECT previous_hard_state FROM previous_hard_state WHERE notification_id > ? ORDER BY notification_id",
		lni,
	)

	nh := bulkInsert{
		stmt: "REPLACE INTO notification_history(id, environment_id, endpoint_id, object_type, host_id, service_id, " +
			"notification_id, type, send_time, state, previous_hard_state, author, `text`, users_notified) " +
			"VALUES (?, ?, ?, ?, ?, ?, X'0000000000000000000000000000000000000000', ?, ?, ?, ?, '-', ?, ?)",
	}

	h := bulkInsert{
		stmt: "REPLACE INTO history(id, environment_id, endpoint_id, object_type, host_id, service_id, " +
			"notification_history_id, event_type, event_time) VALUES (?, ?, ?, ?, ?, ?, ?, 'notification', ?)",
	}

	unh := bulkInsert{
		stmt: "REPLACE INTO user_notification_history(id, environment_id, notification_history_id, user_id) " +
			"VALUES (?, ?, ?, ?)",
	}

	var lastInserted uint64
	massRander := bufio.NewReader(rand.Reader)

	{
		ch := make(chan struct {
			NotificationId     uint64
			NotificationReason uint8
			EndTime            int64

			EndTimeUsec      uint32
			State            uint8
			Output           string
			LongOutput       string
			ContactsNotified uint16

			MonObjecttypeId uint8
			MonObjectName1  string
			MonObjectName2  string

			UserName string
		}, chSize)

		go streamQuery(
			snapshot,
			ch,
			"SELECT n.notification_id, n.notification_reason, UNIX_TIMESTAMP(n.end_time), "+
				"n.end_time_usec, n.state, n.output, n.long_output, n.contacts_notified, "+
				"mo.objecttype_id, mo.name1, IFNULL(mo.name2, ''), "+
				"uo.name1 "+
				"FROM icinga_notifications n "+
				"INNER JOIN icinga_objects mo ON mo.object_id=n.object_id "+
				"INNER JOIN icinga_contactnotifications cn ON cn.notification_id=n.notification_id "+
				"INNER JOIN icinga_objects uo ON uo.object_id=cn.contact_object_id "+
				"WHERE n.notification_id BETWEEN ? AND ? "+
				"ORDER BY n.notification_id",
			lni+1, limit[0].NotificationId,
		)

		for row := range ch {
			id := mkDeterministicUuid(notificationHistory, row.NotificationId)

			if row.NotificationId != lastInserted {
				lastInserted = row.NotificationId

				if len(nh.rows) == *bulk {
					flush(nh, h, unh)

					nh.rows = nil
					h.rows = nil
					unh.rows = nil
				}

				monObjTyp := objectTypes[row.MonObjecttypeId]
				hostId := calcObjectId(row.MonObjectName1)
				serviceId := calcServiceId(row.MonObjectName1, row.MonObjectName2)
				ts := convertTime(row.EndTime, row.EndTimeUsec)

				var typ string
				if row.NotificationReason == 0 {
					if row.State == 0 {
						typ = "recovery"
					} else {
						typ = "problem"
					}
				} else {
					typ = notificationTypes[row.NotificationReason]
				}

				text := row.Output
				if row.LongOutput == "" {
					text += "\n\n" + row.LongOutput
				}

				nh.rows = append(nh.rows, []interface{}{
					id, envId, endpointId, monObjTyp, hostId, serviceId, typ, ts, row.State,
					(<-previousHardStates).PreviousHardState, text, row.ContactsNotified,
				})

				h.rows = append(h.rows, []interface{}{id, envId, endpointId, monObjTyp, hostId, serviceId, id, ts})

				bar.Increment()
			}

			userId := calcObjectId(row.UserName)
			unh.rows = append(unh.rows, []interface{}{mkRandomUuid(massRander), envId, id, userId})
		}

		<-ch
	}

	if len(nh.rows) > 0 {
		flush(nh, h, unh)
	}

	syncBar.stopWorker()
}

// syncStates migrates IDO's icinga_statehistory table to Icinga DB's state_history table using the -sh-cache FILE.
func syncStates() {
	// Icinga DB's state_history#previous_hard_state would need a subquery.
	// That make the IDO reading even slower than the Icinga DB writing.
	// Therefore: Stream IDO's icinga_statehistory once, compute state_history#previous_hard_state
	// and cache it into an SQLite database. Then steam from that database and the IDO.

	dbc, errOp := sql.Open("sqlite3", "file:"+sHcache.value)
	assert(errOp, "Couldn't open SQLite3 database", log.Fields{"file": sHcache.value})
	defer dbc.Close()

	cach := &database{
		whichOne: "-sh-cache",
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

	total, done, lsi := getProgress(
		snapshot, "icinga_statehistory", "statehistory_id", "state_history", "id",
		func(idoId uint64) interface{} { return mkDeterministicUuid(stateHistory, idoId) },
	)

	var limit []struct{ StatehistoryId sql.NullInt64 }

	{
		bar := cacheBar.startWorker(int(total))
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

		{
			ch := make(chan struct {
				StatehistoryId uint64
				ObjectId       uint64
				LastHardState  uint8
			}, chSize)

			// We continue where we finished before. As we build the cache in reverse chronological order:
			// 1. If the history grows between two migration trials, we won't migrate the difference. Workarounds:
			//    a. Start migration after Icinga DB is up and running.
			//    b. Remove the -sh-cache FILE before the next migration trial.
			// 2. If the history gets cleaned up between two migration trials,
			//    the difference either just doesn't appear in the cache or - if already there - will be ignored later.

			go streamQuery(
				snapshot,
				ch,
				"SELECT statehistory_id, object_id, last_hard_state FROM icinga_statehistory "+
					"WHERE statehistory_id < ? ORDER BY statehistory_id DESC",
				checkpoint,
			)

			for row := range ch {
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
			}

			<-ch
		}

		tx.exec(
			"INSERT INTO previous_hard_state(statehistory_id, previous_hard_state) " +
				"SELECT statehistory_id, 99 FROM next_ids",
		)

		tx.exec("DELETE FROM next_hard_state")
		tx.exec("DELETE FROM next_ids")

		tx.fetchAll(&limit, "SELECT MAX(statehistory_id) FROM previous_hard_state")

		tx.commit()
		cach.exec("VACUUM")

		cacheBar.stopWorker()
	}

	bar := syncBar.startWorker(int(total))
	bar.Add(int(done))

	previousHardStates := make(chan struct{ PreviousHardState uint8 }, chSize)

	// Stream concurrently from two databases. Possible due to WHERE and ORDER BY.
	go streamQuery(
		cach,
		previousHardStates,
		"SELECT previous_hard_state FROM previous_hard_state WHERE statehistory_id > ? ORDER BY statehistory_id",
		lsi,
	)

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

	{
		ch := make(chan struct {
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
		}, chSize)

		go streamQuery(
			snapshot,
			ch,
			"SELECT sh.statehistory_id, UNIX_TIMESTAMP(sh.state_time), sh.state_time_usec, "+
				"sh.state_change, sh.state, sh.state_type, sh.current_check_attempt, sh.max_check_attempts, "+
				"sh.last_state, sh.last_hard_state, sh.output, sh.long_output, sh.check_source, "+
				"o.objecttype_id, o.name1, IFNULL(o.name2, '') "+
				"FROM icinga_statehistory sh "+
				"INNER JOIN icinga_objects o ON o.object_id=sh.object_id "+
				"WHERE sh.statehistory_id BETWEEN ? AND ? "+
				"ORDER BY sh.statehistory_id",
			lsi+1, limit[0].StatehistoryId,
		)

		for row := range ch {
			id := mkDeterministicUuid(stateHistory, row.StatehistoryId)
			typ := objectTypes[row.ObjecttypeId]
			hostId := calcObjectId(row.Name1)
			serviceId := calcServiceId(row.Name1, row.Name2)
			ts := convertTime(row.StateTime, row.StateTimeUsec)

			sh.rows = append(sh.rows, []interface{}{
				id, envId, endpointId, typ, hostId, serviceId, ts, stateTypes[row.StateType], row.State,
				row.LastHardState, row.LastState, (<-previousHardStates).PreviousHardState, row.CurrentCheckAttempt,
				row.Output, row.LongOutput, row.MaxCheckAttempts, row.CheckSource,
			})

			h.rows = append(h.rows, []interface{}{id, envId, endpointId, typ, hostId, serviceId, id, ts})

			if len(sh.rows) == *bulk {
				flush(sh, h)

				sh.rows = nil
				h.rows = nil
			}

			bar.Increment()
		}

		<-ch
	}

	if len(sh.rows) > 0 {
		flush(sh, h)
	}

	syncBar.stopWorker()
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
