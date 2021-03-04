package main

import (
	"bufio"
	"crypto/rand"
	"database/sql"
	"github.com/Icinga/icingadb/utils"
	"strings"
)

// syncAcks migrates IDO's icinga_acknowledgements table to Icinga DB's acknowledgement_history table
// using the -ah-cache FILE similar to syncFlapping.
func syncAcks() {
	cach := mkCache("acks")
	defer cach.conn.Close()

	cach.exec(`CREATE TABLE IF NOT EXISTS ack_clear_set_time (
	acknowledgement_id INT PRIMARY KEY,
	entry_time INT,
	entry_time_usec INT
)`)

	cach.exec(`CREATE TABLE IF NOT EXISTS last_ack_set_time (
	object_id INT PRIMARY KEY,
	entry_time INT NOT NULL,
	entry_time_usec INT NOT NULL
)`)

	snapshot := ido.begin(sql.LevelRepeatableRead, true)
	defer snapshot.commit()

	total, done, lai := getProgress(
		snapshot, "icinga_acknowledgements", "acknowledgement_id", "history", "id",
		func(idoId uint64) interface{} { return mkDeterministicUuid(ackHistory, idoId) },
	)

	{
		tx := cach.begin(sql.LevelSerializable, false)
		inTx := 0

		{
			var checkpoint []struct {
				Count int64
				Max   sql.NullInt64
			}
			tx.fetchAll(&checkpoint, "SELECT COUNT(*), MAX(acknowledgement_id) FROM ack_clear_set_time")

			bar := cacheBar.startWorker(total, checkpoint[0].Count*2)

			ch := make(chan struct {
				AcknowledgementId   uint64
				EntryTime           int64
				EntryTimeUsec       uint32
				AcknowledgementType uint8
				ObjectId            uint64
			}, chSize)

			go streamQuery(
				snapshot,
				ch,
				"SELECT ah.acknowledgement_id, UNIX_TIMESTAMP(ah.entry_time), "+
					"ah.entry_time_usec, ah.acknowledgement_type, ah.object_id "+
					"FROM icinga_acknowledgements ah "+
					"INNER JOIN icinga_objects o ON o.object_id=ah.object_id "+
					"WHERE ah.acknowledgement_id > ? "+
					"ORDER BY ah.acknowledgement_id",
				checkpoint[0].Max.Int64,
			)

			for row := range ch {
				if row.AcknowledgementType == 0 {
					var last []struct {
						EntryTime     int64
						EntryTimeUsec uint32
					}
					tx.fetchAll(
						&last,
						"SELECT entry_time, entry_time_usec FROM last_ack_set_time WHERE object_id=?",
						row.ObjectId,
					)

					if len(last) > 0 {
						tx.exec(
							"INSERT INTO ack_clear_set_time(acknowledgement_id, entry_time, entry_time_usec) "+
								"VALUES (?, ?, ?)",
							row.AcknowledgementId, last[0].EntryTime, last[0].EntryTimeUsec,
						)

						tx.exec("DELETE FROM last_ack_set_time WHERE object_id=?", row.ObjectId)
					} else {
						tx.exec(
							"INSERT INTO ack_clear_set_time(acknowledgement_id, entry_time, entry_time_usec) "+
								"VALUES (?, NULL, NULL)",
							row.AcknowledgementId,
						)
					}
				} else {
					tx.exec("DELETE FROM last_ack_set_time WHERE object_id=?", row.ObjectId)

					tx.exec(
						"INSERT INTO last_ack_set_time(object_id, entry_time, entry_time_usec) VALUES (?, ?, ?)",
						row.ObjectId, row.EntryTime, row.EntryTimeUsec,
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

			<-ch // wait for close
		}

		tx.exec("DELETE FROM last_ack_set_time")

		tx.commit()
		cach.exec("VACUUM")

		cacheBar.stopWorker()

		bar := syncBar.startWorker(total, done)

		ackClearSetTime := make(chan struct {
			EntryTime     sql.NullInt64
			EntryTimeUsec sql.NullInt64
		}, chSize)

		go streamQuery(
			cach,
			ackClearSetTime,
			"SELECT entry_time, entry_time_usec "+
				"FROM ack_clear_set_time "+
				"WHERE acknowledgement_id > ? "+
				"ORDER BY acknowledgement_id",
			lai,
		)

		ahs := bulkInsert{
			stmt: "REPLACE INTO acknowledgement_history(id, environment_id, endpoint_id, object_type, host_id, " +
				"service_id, set_time, author, comment, expire_time, is_sticky, is_persistent) " +
				"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		}

		ahc := bulkInsert{stmt: "UPDATE acknowledgement_history SET clear_time=? WHERE id=?"}

		h := bulkInsert{
			stmt: "REPLACE INTO history(id, environment_id, endpoint_id, object_type, host_id, service_id, " +
				"acknowledgement_history_id, event_type, event_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		}

		{
			ch := make(chan struct {
				AcknowledgementId uint64
				EntryTime         int64

				EntryTimeUsec       uint32
				AcknowledgementType uint8
				AuthorName          string
				CommentData         string

				IsSticky          uint8
				PersistentComment uint8
				EndTime           sql.NullInt64

				ObjecttypeId uint8
				Name1        string
				Name2        string
			}, chSize)

			go streamQuery(
				snapshot,
				ch,
				"SELECT ah.acknowledgement_id, UNIX_TIMESTAMP(ah.entry_time), "+
					"ah.entry_time_usec, ah.acknowledgement_type, ah.author_name, ah.comment_data, "+
					"ah.is_sticky, ah.persistent_comment, UNIX_TIMESTAMP(ah.end_time), "+
					"o.objecttype_id, o.name1, IFNULL(o.name2, '') "+
					"FROM icinga_acknowledgements ah "+
					"INNER JOIN icinga_objects o ON o.object_id=ah.object_id "+
					"WHERE ah.acknowledgement_id > ? "+
					"ORDER BY ah.acknowledgement_id",
				lai,
			)

			for row := range ch {
				name := row.Name1
				if row.Name2 != "" {
					name += "!" + row.Name2
				}

				ts := convertTime(row.EntryTime, row.EntryTimeUsec)
				var set uint64

				if row.AcknowledgementType == 0 {
					st := <-ackClearSetTime
					if !st.EntryTime.Valid {
						continue
					}

					set = convertTime(st.EntryTime.Int64, uint32(st.EntryTimeUsec.Int64))
				} else {
					set = ts
				}

				id := mkDeterministicUuid(ackHistory, row.AcknowledgementId)
				typ := objectTypes[row.ObjecttypeId]
				hostId := calcObjectId(row.Name1)
				serviceId := calcServiceId(row.Name1, row.Name2)

				acknowledgementHistoryId := hashAny([]interface{}{
					icingaEnv.value, strings.Title(objectTypes[row.ObjecttypeId]), name, set,
				})

				if row.AcknowledgementType == 0 {
					ahc.rows = append(ahc.rows, []interface{}{ts, acknowledgementHistoryId})

					h.rows = append(h.rows, []interface{}{
						id, envId, endpointId, typ, hostId, serviceId, acknowledgementHistoryId, "ack_clear", ts,
					})
				} else {
					row.EndTime.Int64 *= 1000

					ahs.rows = append(ahs.rows, []interface{}{
						acknowledgementHistoryId, envId, endpointId, typ, hostId,
						serviceId, set, row.AuthorName, row.CommentData, row.EndTime,
						bools[row.IsSticky], bools[row.PersistentComment],
					})

					h.rows = append(h.rows, []interface{}{
						id, envId, endpointId, typ, hostId, serviceId, acknowledgementHistoryId, "ack_set", ts,
					})
				}

				if len(h.rows) == *bulk {
					flush(ahs, ahc, h)

					ahs.rows = nil
					ahc.rows = nil
					h.rows = nil
				}

				bar.Increment()
			}

			<-ch // wait for close
		}

		if len(h.rows) > 0 {
			flush(ahs, ahc, h)
		}

		syncBar.stopWorker()
	}
}

// syncComments migrates IDO's icinga_commenthistory table to Icinga DB's comment_history table.
func syncComments() {
	snapshot := ido.begin(sql.LevelRepeatableRead, true)
	defer snapshot.commit()

	total, done, lci := getProgress(
		snapshot, "icinga_commenthistory", "commenthistory_id", "comment_history", "comment_id",
		func(idoId uint64) interface{} {
			var res []struct{ Name string }
			snapshot.fetchAll(&res, "SELECT name FROM icinga_commenthistory WHERE commenthistory_id=?", idoId)

			if len(res) < 1 {
				return make([]byte, 20)
			} else {
				return calcObjectId(res[0].Name)
			}
		},
	)

	bar := syncBar.startWorker(total, done)

	coh := bulkInsert{
		stmt: "REPLACE INTO comment_history(comment_id, environment_id, endpoint_id, object_type, host_id, " +
			"service_id, entry_time, author, comment, entry_type, is_persistent, is_sticky, expire_time, " +
			"remove_time, has_been_removed) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'n', ?, ?, ?)",
	}

	h := bulkInsert{
		stmt: "REPLACE INTO history(id, environment_id, endpoint_id, object_type, host_id, service_id, " +
			"comment_history_id, event_type, event_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
	}

	{
		ch := make(chan struct {
			EntryTime     int64
			EntryTimeUsec uint32
			EntryType     uint8
			AuthorName    string

			CommentData    string
			IsPersistent   uint8
			Expires        uint8
			ExpirationTime int64

			DeletionTime     int64
			DeletionTimeUsec uint32
			Name             string

			MonObjecttypeId uint8
			MonObjName1     string
			MonObjName2     string
		}, chSize)

		go streamQuery(
			snapshot,
			ch,
			"SELECT UNIX_TIMESTAMP(ch.entry_time), ch.entry_time_usec, ch.entry_type, ch.author_name, "+
				"ch.comment_data, ch.is_persistent, ch.expires, IFNULL(UNIX_TIMESTAMP(ch.expiration_time), 0), "+
				"IFNULL(UNIX_TIMESTAMP(ch.deletion_time), 0), ch.deletion_time_usec, ch.name, "+
				"o.objecttype_id, o.name1, IFNULL(o.name2, '') "+
				"FROM icinga_commenthistory ch "+
				"INNER JOIN icinga_objects o ON o.object_id=ch.object_id "+
				"WHERE ch.commenthistory_id > ? "+
				"ORDER BY ch.commenthistory_id",
			lci,
		)

		massRander := bufio.NewReader(rand.Reader)

		for row := range ch {
			id := calcObjectId(row.Name)
			typ := objectTypes[row.MonObjecttypeId]
			hostId := calcObjectId(row.MonObjName1)
			serviceId := calcServiceId(row.MonObjName1, row.MonObjName2)
			removeTime := convertTime(row.DeletionTime, row.DeletionTimeUsec)

			coh.rows = append(coh.rows, []interface{}{
				id, envId, endpointId, typ, hostId, serviceId, row.EntryTime * 1000, row.AuthorName,
				row.CommentData, commentTypes[row.EntryType], bools[row.IsPersistent],
				row.ExpirationTime * 1000, removeTime, utils.Bool[removeTime == 0],
			})

			h.rows = append(h.rows, []interface{}{
				mkRandomUuid(massRander), envId, endpointId, typ, hostId,
				serviceId, id, "comment_add", row.EntryTime * 1000,
			})

			if removeTime != 0 {
				h.rows = append(h.rows, []interface{}{
					mkRandomUuid(massRander), envId, endpointId, typ, hostId,
					serviceId, id, "comment_remove", removeTime,
				})
			}

			if len(coh.rows) == *bulk {
				flush(coh, h)

				coh.rows = nil
				h.rows = nil
			}

			bar.Increment()
		}

		<-ch // wait for close
	}

	if len(coh.rows) > 0 {
		flush(coh, h)
	}

	syncBar.stopWorker()
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

	bar := syncBar.startWorker(total, done)

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
				"UNIX_TIMESTAMP(dh.scheduled_start_time), IFNULL(UNIX_TIMESTAMP(dh.scheduled_end_time), 0), "+
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

		<-ch // wait for close
	}

	if len(dh.rows) > 0 {
		flush(dh, h)
	}

	syncBar.stopWorker()
}

// syncFlapping migrates IDO's icinga_flappinghistory table to Icinga DB's flapping_history table
// using the -fh-cache FILE.
func syncFlapping() {
	// Icinga DB's flapping_history#id always needs start_time. flapping_end rows would need a subquery for that.
	// That would make the IDO reading even slower than the Icinga DB writing.
	// Therefore: Stream IDO's icinga_flappinghistory once, compute flapping_history#start_time
	// and cache it into an SQLite database. Then steam from that database and the IDO.

	cach := mkCache("flapping")
	defer cach.conn.Close()

	// Icinga DB's flapping_history#start_time per flapping_end row (IDO's icinga_flappinghistory#flappinghistory_id).
	cach.exec(`CREATE TABLE IF NOT EXISTS flapping_end_start_time (
	flappinghistory_id INT PRIMARY KEY,
	event_time INT,
	event_time_usec INT
)`)

	// Helper table, the last start_time per icinga_statehistory#object_id.
	cach.exec(`CREATE TABLE IF NOT EXISTS last_flapping_start_time (
	object_id INT PRIMARY KEY,
	event_time INT NOT NULL,
	event_time_usec INT NOT NULL
)`)

	snapshot := ido.begin(sql.LevelRepeatableRead, true)
	defer snapshot.commit()

	total, done, lfi := getProgress(
		snapshot, "icinga_flappinghistory", "flappinghistory_id", "history", "id",
		func(idoId uint64) interface{} { return mkDeterministicUuid(flappingHistory, idoId) },
	)

	{
		tx := cach.begin(sql.LevelSerializable, false)
		inTx := 0

		{
			var checkpoint []struct {
				Count int64
				Max   sql.NullInt64
			}
			tx.fetchAll(&checkpoint, "SELECT COUNT(*), MAX(flappinghistory_id) FROM flapping_end_start_time")

			bar := cacheBar.startWorker(total, checkpoint[0].Count*2)

			ch := make(chan struct {
				FlappinghistoryId uint64
				EventTime         int64
				EventTimeUsec     uint32
				EventType         uint16
				ObjectId          uint64
			}, chSize)

			go streamQuery(
				snapshot,
				ch,
				"SELECT fh.flappinghistory_id, UNIX_TIMESTAMP(fh.event_time), "+
					"fh.event_time_usec, fh.event_type, fh.object_id "+
					"FROM icinga_flappinghistory fh "+
					"INNER JOIN icinga_objects o ON o.object_id=fh.object_id "+
					"WHERE fh.flappinghistory_id > ? "+
					"ORDER BY fh.flappinghistory_id",
				checkpoint[0].Max.Int64,
			)

			for row := range ch {
				if row.EventType == flappingStart {
					tx.exec("DELETE FROM last_flapping_start_time WHERE object_id=?", row.ObjectId)

					tx.exec(
						"INSERT INTO last_flapping_start_time(object_id, event_time, event_time_usec) VALUES (?, ?, ?)",
						row.ObjectId, row.EventTime, row.EventTimeUsec,
					)
				} else {
					var lfst []struct {
						EventTime     int64
						EventTimeUsec uint32
					}
					tx.fetchAll(
						&lfst,
						"SELECT event_time, event_time_usec FROM last_flapping_start_time WHERE object_id=?",
						row.ObjectId,
					)

					if len(lfst) > 0 {
						tx.exec(
							"INSERT INTO flapping_end_start_time(flappinghistory_id, event_time, event_time_usec) "+
								"VALUES (?, ?, ?)",
							row.FlappinghistoryId, lfst[0].EventTime, lfst[0].EventTimeUsec,
						)

						tx.exec("DELETE FROM last_flapping_start_time WHERE object_id=?", row.ObjectId)
					} else {
						tx.exec(
							"INSERT INTO flapping_end_start_time(flappinghistory_id, event_time, event_time_usec) "+
								"VALUES (?, NULL, NULL)",
							row.FlappinghistoryId,
						)
					}
				}

				inTx++
				if inTx == *bulk {
					tx.commit()
					tx = cach.begin(sql.LevelSerializable, false)
					inTx = 0
				}

				bar.Increment()
			}

			<-ch // wait for close
		}

		tx.exec("DELETE FROM last_flapping_start_time")

		tx.commit()
		cach.exec("VACUUM")

		cacheBar.stopWorker()

		bar := syncBar.startWorker(total, done)

		flappingEndStartTimes := make(chan struct {
			EventTime     sql.NullInt64
			EventTimeUsec sql.NullInt64
		}, chSize)

		// Stream concurrently from two databases. Possible due to WHERE and ORDER BY.
		go streamQuery(
			cach,
			flappingEndStartTimes,
			"SELECT event_time, event_time_usec "+
				"FROM flapping_end_start_time "+
				"WHERE flappinghistory_id > ? "+
				"ORDER BY flappinghistory_id",
			lfi,
		)

		fhs := bulkInsert{
			stmt: "REPLACE INTO flapping_history(id, environment_id, endpoint_id, object_type, host_id, service_id, " +
				"start_time, percent_state_change_start, flapping_threshold_low, " +
				"flapping_threshold_high) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		}

		fhe := bulkInsert{
			stmt: "UPDATE flapping_history " +
				"SET end_time=?, percent_state_change_end=?, flapping_threshold_low=?, flapping_threshold_high=? " +
				"WHERE id=?",
		}

		h := bulkInsert{
			stmt: "REPLACE INTO history(id, environment_id, endpoint_id, object_type, host_id, service_id, " +
				"flapping_history_id, event_type, event_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		}

		{
			ch := make(chan struct {
				FlappinghistoryId uint64
				EventTime         int64
				EventTimeUsec     uint32

				EventType          uint16
				PercentStateChange float64
				LowThreshold       float64
				HighThreshold      float64

				ObjecttypeId uint8
				Name1        string
				Name2        string
			}, chSize)

			go streamQuery(
				snapshot,
				ch,
				"SELECT fh.flappinghistory_id, UNIX_TIMESTAMP(fh.event_time), fh.event_time_usec, "+
					"fh.event_type, fh.percent_state_change, fh.low_threshold, fh.high_threshold, "+
					"o.objecttype_id, o.name1, IFNULL(o.name2, '') "+
					"FROM icinga_flappinghistory fh "+
					"INNER JOIN icinga_objects o ON o.object_id=fh.object_id "+
					"WHERE fh.flappinghistory_id > ? "+
					"ORDER BY fh.flappinghistory_id",
				lfi,
			)

			for row := range ch {
				name := row.Name1
				if row.Name2 != "" {
					name += "!" + row.Name2
				}

				ts := convertTime(row.EventTime, row.EventTimeUsec)
				var start uint64

				if row.EventType == flappingStart {
					start = ts
				} else {
					st := <-flappingEndStartTimes
					if !st.EventTime.Valid {
						// End w/o associated start.
						continue
					}

					start = convertTime(st.EventTime.Int64, uint32(st.EventTimeUsec.Int64))
				}

				id := mkDeterministicUuid(flappingHistory, row.FlappinghistoryId)
				flappingHistoryId := hashAny([]interface{}{icingaEnv.value, strings.Title(objectTypes[row.ObjecttypeId]), name, start})
				typ := objectTypes[row.ObjecttypeId]
				hostId := calcObjectId(row.Name1)
				serviceId := calcServiceId(row.Name1, row.Name2)

				if row.EventType == flappingStart {
					fhs.rows = append(fhs.rows, []interface{}{
						flappingHistoryId, envId, endpointId, typ, hostId, serviceId,
						start, row.PercentStateChange, row.LowThreshold, row.HighThreshold,
					})

					h.rows = append(h.rows, []interface{}{
						id, envId, endpointId, typ, hostId, serviceId, flappingHistoryId, "flapping_start", ts,
					})
				} else {
					fhe.rows = append(fhe.rows, []interface{}{
						ts, row.PercentStateChange, row.LowThreshold, row.HighThreshold, flappingHistoryId,
					})

					h.rows = append(h.rows, []interface{}{
						id, envId, endpointId, typ, hostId, serviceId, flappingHistoryId, "flapping_end", ts,
					})
				}

				if len(h.rows) == *bulk {
					flush(fhs, fhe, h)

					fhs.rows = nil
					fhe.rows = nil
					h.rows = nil
				}

				bar.Increment()
			}

			<-ch // wait for close
		}

		if len(h.rows) > 0 {
			flush(fhs, fhe, h)
		}

		syncBar.stopWorker()
	}
}

// syncNotifications migrates IDO's icinga_notifications table
// to Icinga DB's notification_history table using the -nh-cache FILE similar to syncStates.
func syncNotifications() {
	cach := mkCache("notifications")
	defer cach.conn.Close()

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
		tx := cach.begin(sql.LevelSerializable, false)
		var checkpoint int64

		var niCMNi []struct {
			Count          int64
			NotificationId sql.NullInt64
		}
		tx.fetchAll(&niCMNi, "SELECT COUNT(*), MIN(notification_id) FROM next_ids")

		var phsC []struct{ Count int64 }
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

		bar := cacheBar.startWorker(total, phsC[0].Count+niCMNi[0].Count)
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
				"SELECT nh.notification_id, nh.object_id, nh.state "+
					"FROM icinga_notifications nh INNER JOIN icinga_objects o ON o.object_id=nh.object_id "+
					"WHERE nh.notification_id < ? ORDER BY nh.notification_id DESC",
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

			<-ch // wait for close
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

	bar := syncBar.startWorker(total, done)
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

		<-ch // wait for close
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

	cach := mkCache("states")
	defer cach.conn.Close()

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
		tx := cach.begin(sql.LevelSerializable, false)
		var checkpoint int64

		var niCMShi []struct {
			Count          int64
			StatehistoryId sql.NullInt64
		}
		tx.fetchAll(&niCMShi, "SELECT COUNT(*), MIN(statehistory_id) FROM next_ids")

		var phsC []struct{ Count int64 }
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

		bar := cacheBar.startWorker(total, phsC[0].Count+niCMShi[0].Count)
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
				"SELECT sh.statehistory_id, sh.object_id, sh.last_hard_state "+
					"FROM icinga_statehistory sh INNER JOIN icinga_objects o ON o.object_id=sh.object_id "+
					"WHERE sh.statehistory_id < ? ORDER BY sh.statehistory_id DESC",
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

			<-ch // wait for close
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

	bar := syncBar.startWorker(total, done)
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

		<-ch // wait for close
	}

	if len(sh.rows) > 0 {
		flush(sh, h)
	}

	syncBar.stopWorker()
}
