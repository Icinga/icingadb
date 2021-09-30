package main

import (
	"database/sql"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"strings"
	"time"
)

var eventTimeCacheSchema = []string{
	// Icinga DB's flapping_history#start_time per flapping_end row (IDO's icinga_flappinghistory#flappinghistory_id).
	`CREATE TABLE IF NOT EXISTS end_start_time (
	history_id INT PRIMARY KEY,
	event_time INT NOT NULL,
	event_time_usec INT NOT NULL
)`,
	// Helper table, the last start_time per icinga_statehistory#object_id.
	`CREATE TABLE IF NOT EXISTS last_start_time (
	object_id INT PRIMARY KEY,
	event_time INT NOT NULL,
	event_time_usec INT NOT NULL
)`,
}

// buildEventTimeCache rationale:
//
// Icinga DB's flapping_history#id always needs start_time. flapping_end rows would need an IDO subquery for that.
// That would make the IDO reading even slower than the Icinga DB writing.
// Therefore: Stream IDO's icinga_flappinghistory once, compute flapping_history#start_time
// and cache it into an SQLite database. Then steam from that database and the IDO.
//
// Similar for acknowledgements.
func buildEventTimeCache(ht *historyType, idoColumns []string) {
	chunkCacheTx(ht.cache, func(tx **sqlx.Tx, onDeleted func(sql.Result), onNewUncommittedDml func()) {
		var checkpoint struct {
			Cnt   int64
			MaxId sql.NullInt64
		}
		cacheGet(*tx, &checkpoint, "SELECT COUNT(*) cnt, MAX(history_id) max_id FROM end_start_time")

		ht.bar.SetCurrent(checkpoint.Cnt * 2)
		inc := barIncrementer{ht.bar, time.Now()}

		sliceIdoHistory(
			ht.snapshot,
			"SELECT "+strings.Join(idoColumns, ", ")+" FROM "+ht.idoTable+
				" xh USE INDEX (PRIMARY) INNER JOIN icinga_objects o ON o.object_id=xh.object_id WHERE xh."+
				ht.idoIdColumn+" > ? ORDER BY xh."+ht.idoIdColumn+" LIMIT ?",
			nil, checkpoint.MaxId.Int64,
			func(idoRows []struct {
				Id            uint64
				EventTime     int64
				EventTimeUsec uint32
				EventIsStart  uint8
				ObjectId      uint64
			}) (checkpoint interface{}) {
				for _, idoRow := range idoRows {
					if idoRow.EventIsStart == 0 {
						var lst []struct {
							EventTime     int64
							EventTimeUsec uint32
						}
						cacheSelect(
							*tx, &lst, "SELECT event_time, event_time_usec FROM last_start_time WHERE object_id=?",
							idoRow.ObjectId,
						)

						if len(lst) > 0 {
							cacheExec(
								*tx, false,
								"INSERT INTO end_start_time(history_id, event_time, event_time_usec) VALUES (?, ?, ?)",
								idoRow.Id, lst[0].EventTime, lst[0].EventTimeUsec,
							)

							onDeleted(cacheExec(
								*tx, false, "DELETE FROM last_start_time WHERE object_id=?", idoRow.ObjectId,
							))
						}
					} else {
						onDeleted(cacheExec(
							*tx, false, "DELETE FROM last_start_time WHERE object_id=?", idoRow.ObjectId,
						))

						cacheExec(
							*tx, false,
							"INSERT INTO last_start_time(object_id, event_time, event_time_usec) VALUES (?, ?, ?)",
							idoRow.ObjectId, idoRow.EventTime, idoRow.EventTimeUsec,
						)
					}

					onNewUncommittedDml()
					checkpoint = idoRow.Id
				}

				inc.inc(len(idoRows))
				return
			},
		)

		onDeleted(cacheExec(*tx, false, "DELETE FROM last_start_time"))
	})

	ht.bar.SetTotal(ht.bar.Current(), true)
}

var previousHardStateCacheSchema = []string{
	// Icinga DB's state_history#previous_hard_state per IDO's icinga_statehistory#statehistory_id.
	`CREATE TABLE IF NOT EXISTS previous_hard_state (
	history_id INT PRIMARY KEY,
	previous_hard_state INT NOT NULL
)`,
	// Helper table, the current last_hard_state per icinga_statehistory#object_id.
	`CREATE TABLE IF NOT EXISTS next_hard_state (
	object_id INT PRIMARY KEY,
	next_hard_state INT NOT NULL
)`,
	// Helper table for stashing icinga_statehistory#statehistory_id until last_hard_state changes.
	`CREATE TABLE IF NOT EXISTS next_ids (
	object_id INT NOT NULL,
	history_id INT NOT NULL
)`,
	"CREATE INDEX IF NOT EXISTS next_ids_object_id ON next_ids(object_id)",
	"CREATE INDEX IF NOT EXISTS next_ids_history_id ON next_ids(history_id)",
}

// buildPreviousHardStateCache rationale:
//
// Icinga DB's state_history#previous_hard_state would need a subquery.
// That make the IDO reading even slower than the Icinga DB writing.
// Therefore: Stream IDO's icinga_statehistory once, compute state_history#previous_hard_state
// and cache it into an SQLite database. Then steam from that database and the IDO.
//
// Similar for notifications.
func buildPreviousHardStateCache(ht *historyType, idoColumns []string) {
	chunkCacheTx(ht.cache, func(tx **sqlx.Tx, onDeleted func(sql.Result), onNewUncommittedDml func()) {
		var nextIds struct {
			Cnt   int64
			MinId sql.NullInt64
		}
		cacheGet(*tx, &nextIds, "SELECT COUNT(*) cnt, MIN(history_id) min_id FROM next_ids")

		var previousHardState struct{ Cnt int64 }
		cacheGet(*tx, &previousHardState, "SELECT COUNT(*) cnt FROM previous_hard_state")

		var checkpoint int64
		if nextIds.MinId.Valid {
			checkpoint = nextIds.MinId.Int64
		} else {
			// next_ids contains the most recently processed IDs and is only empty if...
			if previousHardState.Cnt == 0 {
				// ... we didn't actually start yet...
				checkpoint = (1 << 63) - 1
			} else {
				// ... or we've already finished.
				checkpoint = 0
			}
		}

		ht.bar.SetCurrent(previousHardState.Cnt + nextIds.Cnt)
		inc := barIncrementer{ht.bar, time.Now()}

		// We continue where we finished before. As we build the cache in reverse chronological order:
		// 1. If the history grows between two migration trials, we won't migrate the difference. Workarounds:
		//    a. Start migration after Icinga DB is up and running.
		//    b. Remove the cache before the next migration trial.
		// 2. If the history gets cleaned up between two migration trials,
		//    the difference either just doesn't appear in the cache or - if already there - will be ignored later.

		sliceIdoHistory(
			ht.snapshot,
			"SELECT "+strings.Join(idoColumns, ", ")+" FROM "+ht.idoTable+
				" xh USE INDEX (PRIMARY) INNER JOIN icinga_objects o ON o.object_id=xh.object_id WHERE xh."+
				ht.idoIdColumn+" < ? ORDER BY xh."+ht.idoIdColumn+" DESC LIMIT ?",
			nil, checkpoint,
			func(idoRows []struct {
				Id            uint64
				ObjectId      uint64
				LastHardState uint8
			}) (checkpoint interface{}) {
				for _, idoRow := range idoRows {
					var nhs []struct{ NextHardState uint8 }
					cacheSelect(*tx, &nhs, "SELECT next_hard_state FROM next_hard_state WHERE object_id=?", idoRow.ObjectId)

					if len(nhs) < 1 {
						cacheExec(
							*tx, false, "INSERT INTO next_hard_state(object_id, next_hard_state) VALUES (?, ?)",
							idoRow.ObjectId, idoRow.LastHardState,
						)

						cacheExec(
							*tx, false, "INSERT INTO next_ids(history_id, object_id) VALUES (?, ?)",
							idoRow.Id, idoRow.ObjectId,
						)
					} else if idoRow.LastHardState == nhs[0].NextHardState {
						cacheExec(
							*tx, false, "INSERT INTO next_ids(history_id, object_id) VALUES (?, ?)",
							idoRow.Id, idoRow.ObjectId,
						)
					} else {
						cacheExec(
							*tx, false, "INSERT INTO previous_hard_state(history_id, previous_hard_state) "+
								"SELECT history_id, ? FROM next_ids WHERE object_id=?",
							idoRow.LastHardState, idoRow.ObjectId,
						)

						onDeleted(cacheExec(*tx, false, "DELETE FROM next_hard_state WHERE object_id=?", idoRow.ObjectId))
						onDeleted(cacheExec(*tx, false, "DELETE FROM next_ids WHERE object_id=?", idoRow.ObjectId))

						cacheExec(
							*tx, false, "INSERT INTO next_hard_state(object_id, next_hard_state) VALUES (?, ?)",
							idoRow.ObjectId, idoRow.LastHardState,
						)

						cacheExec(
							*tx, false, "INSERT INTO next_ids(history_id, object_id) VALUES (?, ?)",
							idoRow.Id, idoRow.ObjectId,
						)
					}

					onNewUncommittedDml()
					checkpoint = idoRow.Id
				}

				inc.inc(len(idoRows))
				return
			},
		)

		cacheExec(
			*tx, false, "INSERT INTO previous_hard_state(history_id, previous_hard_state) "+
				"SELECT history_id, 99 FROM next_ids",
		)

		onDeleted(cacheExec(*tx, false, "DELETE FROM next_hard_state"))
		onDeleted(cacheExec(*tx, false, "DELETE FROM next_ids"))
	})

	ht.bar.SetTotal(ht.bar.Current(), true)
}

func chunkCacheTx(cache *sqlx.DB, do func(tx **sqlx.Tx, onDeleted func(sql.Result), onNewUncommittedDml func())) {
	logger := log.With("backend", "cache")
	var deleted int64
	var inTx int

	tx, err := cache.Beginx()
	if err != nil {
		logger.Fatalf("%+v", errors.Wrap(err, "can't begin transaction"))
	}

	do(
		&tx,
		func(result sql.Result) {
			if affected, err := result.RowsAffected(); err == nil {
				deleted += affected
			} else {
				log.Errorf("%+v", errors.Wrap(err, "can't get affected rows"))
			}
		},
		func() {
			inTx++
			if inTx == bulk {
				if err := tx.Commit(); err != nil {
					logger.Fatalf("%+v", errors.Wrap(err, "can't commit transaction"))
				}

				var err error

				tx, err = cache.Beginx()
				if err != nil {
					logger.Fatalf("%+v", errors.Wrap(err, "can't begin transaction"))
				}

				inTx = 0
			}
		},
	)

	if err := tx.Commit(); err != nil {
		logger.Fatalf("%+v", errors.Wrap(err, "can't commit transaction"))
	}

	if deleted > 0 {
		cacheExec(cache, true, "VACUUM")
	}
}

func cacheGet(cache interface {
	Get(dest interface{}, query string, args ...interface{}) error
}, dest interface{}, query string, args ...interface{}) {
	if err := cache.Get(dest, query, args...); err != nil {
		log.With("backend", "cache", "query", query, "args", args).
			Fatalf("%+v", errors.Wrap(err, "can't perform query"))
	}
}

func cacheSelect(cacheTx *sqlx.Tx, dest interface{}, query string, args ...interface{}) {
	if err := cacheTx.Select(dest, query, args...); err != nil {
		log.With("backend", "cache", "query", query, "args", args).
			Fatalf("%+v", errors.Wrap(err, "can't perform query"))
	}
}

func cacheExec(cache sqlx.Execer, allowFailure bool, dml string, args ...interface{}) sql.Result {
	res, err := cache.Exec(dml, args...)
	if err != nil {
		logger := log.With("backend", "cache", "dml", dml, "args", args)

		level := logger.Fatalf
		if allowFailure {
			level = logger.Errorf
		}

		level("%+v", errors.Wrap(err, "can't perform DML"))
	}

	return res
}
