package main

import (
	"database/sql"
	_ "embed"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"math"
	"strings"
)

//go:embed embed/event_time_cache_schema.sql
var eventTimeCacheSchema string

//go:embed embed/previous_hard_state_cache_schema.sql
var previousHardStateCacheSchema string

// buildEventTimeCache rationale:
//
// Icinga DB's flapping_history#id always needs start_time. flapping_end rows would need an IDO subquery for that.
// That would make the IDO reading even slower than the Icinga DB writing.
// Therefore: Stream IDO's icinga_flappinghistory once, compute flapping_history#start_time
// and cache it into an SQLite database. Then steam from that database and the IDO.
//
// Similar for acknowledgements. (On non-recoverable errors the whole program exits.)
func buildEventTimeCache(ht *historyType, idoColumns []string) {
	type row = struct {
		Id            uint64
		EventTime     int64
		EventTimeUsec uint32
		EventIsStart  uint8
		ObjectId      uint64
	}

	chunkCacheTx(ht.cache, func(tx **sqlx.Tx, commitPeriodically func()) {
		var checkpoint struct {
			Cnt   int64
			MaxId sql.NullInt64
		}
		cacheGet(*tx, &checkpoint, "SELECT COUNT(*) cnt, MAX(history_id) max_id FROM end_start_time")

		ht.bar.SetCurrent(checkpoint.Cnt * 2)

		// Stream source data...
		sliceIdoHistory(
			ht.snapshot,
			"SELECT "+strings.Join(idoColumns, ", ")+" FROM "+ht.idoTable+
				// For actual migration icinga_objects will be joined anyway,
				// so it makes no sense to take vanished objects into account.
				" xh USE INDEX (PRIMARY) INNER JOIN icinga_objects o ON o.object_id=xh.object_id WHERE xh."+
				ht.idoIdColumn+" > :checkpoint ORDER BY xh."+ht.idoIdColumn+" LIMIT :bulk",
			nil, checkpoint.MaxId.Int64, // ... since we were interrupted:
			func(idoRows []row) (checkpoint interface{}) {
				for _, idoRow := range idoRows {
					if idoRow.EventIsStart == 0 {
						// Ack/flapping end event. Get the start event time:
						var lst []struct {
							EventTime     int64
							EventTimeUsec uint32
						}
						cacheSelect(
							*tx, &lst, "SELECT event_time, event_time_usec FROM last_start_time WHERE object_id=?",
							idoRow.ObjectId,
						)

						// If we have that, ...
						if len(lst) > 0 {
							// ... save the start event time for the actual migration:
							cacheExec(
								*tx,
								"INSERT INTO end_start_time(history_id, event_time, event_time_usec) VALUES (?, ?, ?)",
								idoRow.Id, lst[0].EventTime, lst[0].EventTimeUsec,
							)

							// This previously queried info isn't needed anymore.
							cacheExec(*tx, "DELETE FROM last_start_time WHERE object_id=?", idoRow.ObjectId)
						}
					} else {
						// Ack/flapping start event directly after another start event (per checkable).
						// The old one won't have (but the new one will) an end event (which will need its time).
						cacheExec(*tx, "DELETE FROM last_start_time WHERE object_id=?", idoRow.ObjectId)

						// An ack/flapping start event. The following end event (per checkable) will need its time.
						cacheExec(
							*tx, "INSERT INTO last_start_time(object_id, event_time, event_time_usec) VALUES (?, ?, ?)",
							idoRow.ObjectId, idoRow.EventTime, idoRow.EventTimeUsec,
						)
					}

					commitPeriodically()
					checkpoint = idoRow.Id
				}

				ht.bar.IncrBy(len(idoRows))
				return
			},
		)

		// This never queried info isn't needed anymore.
		cacheExec(*tx, "DELETE FROM last_start_time")
	})

	ht.bar.SetTotal(ht.bar.Current(), true)
}

// buildPreviousHardStateCache rationale:
//
// Icinga DB's state_history#previous_hard_state would need a subquery.
// That make the IDO reading even slower than the Icinga DB writing.
// Therefore: Stream IDO's icinga_statehistory once, compute state_history#previous_hard_state
// and cache it into an SQLite database. Then steam from that database and the IDO.
//
// Similar for notifications. (On non-recoverable errors the whole program exits.)
func buildPreviousHardStateCache(ht *historyType, idoColumns []string) {
	type row = struct {
		Id            uint64
		ObjectId      uint64
		LastHardState uint8
	}

	chunkCacheTx(ht.cache, func(tx **sqlx.Tx, commitPeriodically func()) {
		var nextIds struct {
			Cnt   int64
			MinId sql.NullInt64
		}
		cacheGet(*tx, &nextIds, "SELECT COUNT(*) cnt, MIN(history_id) min_id FROM next_ids")

		var previousHardStateCnt int64
		cacheGet(*tx, &previousHardStateCnt, "SELECT COUNT(*) FROM previous_hard_state")

		var checkpoint int64
		if nextIds.MinId.Valid { // there are next_ids
			checkpoint = nextIds.MinId.Int64 // this kind of caches is filled descending
		} else { // there aren't any next_ids
			// next_ids contains the most recently processed IDs and is only empty if...
			if previousHardStateCnt == 0 {
				// ... we didn't actually start yet...
				checkpoint = math.MaxInt64 // start from the largest (possible) ID
			} else {
				// ... or we've already finished.
				checkpoint = 0 // make following query no-op
			}
		}

		ht.bar.SetCurrent(previousHardStateCnt + nextIds.Cnt)

		// We continue where we finished before. As we build the cache in reverse chronological order:
		// 1. If the history grows between two migration trials, we won't migrate the difference. Workarounds:
		//    a. Start migration after Icinga DB is up and running.
		//    b. Remove the cache before the next migration trial.
		// 2. If the history gets cleaned up between two migration trials,
		//    the difference either just doesn't appear in the cache or - if already there - will be ignored later.

		// Stream source data...
		sliceIdoHistory(
			ht.snapshot,
			"SELECT "+strings.Join(idoColumns, ", ")+" FROM "+ht.idoTable+
				// For actual migration icinga_objects will be joined anyway,
				// so it makes no sense to take vanished objects into account.
				" xh USE INDEX (PRIMARY) INNER JOIN icinga_objects o ON o.object_id=xh.object_id WHERE xh."+
				ht.idoIdColumn+" < :checkpoint ORDER BY xh."+ht.idoIdColumn+" DESC LIMIT :bulk",
			nil, checkpoint, // ... since we were interrupted:
			func(idoRows []row) (checkpoint interface{}) {
				for _, idoRow := range idoRows {
					var nhs []struct{ NextHardState uint8 }
					cacheSelect(*tx, &nhs, "SELECT next_hard_state FROM next_hard_state WHERE object_id=?", idoRow.ObjectId)

					if len(nhs) < 1 { // we just started (per checkable)
						// At the moment (we're "travelling back in time") that's the checkable's hard state:
						cacheExec(
							*tx, "INSERT INTO next_hard_state(object_id, next_hard_state) VALUES (?, ?)",
							idoRow.ObjectId, idoRow.LastHardState,
						)

						// But for the current time point the previous hard state isn't known, yet:
						cacheExec(
							*tx, "INSERT INTO next_ids(history_id, object_id) VALUES (?, ?)",
							idoRow.Id, idoRow.ObjectId,
						)
					} else if idoRow.LastHardState == nhs[0].NextHardState {
						// The hard state didn't change yet (per checkable),
						// so this time point also awaits the previous hard state.
						cacheExec(
							*tx, "INSERT INTO next_ids(history_id, object_id) VALUES (?, ?)",
							idoRow.Id, idoRow.ObjectId,
						)
					} else { // the hard state changed (per checkable)
						// That past hard state is now available for the processed future time points:
						cacheExec(
							*tx,
							"INSERT INTO previous_hard_state(history_id, previous_hard_state) "+
								"SELECT history_id, ? FROM next_ids WHERE object_id=?",
							idoRow.LastHardState, idoRow.ObjectId,
						)

						// Now they have what they wanted:
						cacheExec(*tx, "DELETE FROM next_hard_state WHERE object_id=?", idoRow.ObjectId)
						cacheExec(*tx, "DELETE FROM next_ids WHERE object_id=?", idoRow.ObjectId)

						// That's done.
						// Now do the same thing as in the "we just started" case above, for the same reason:

						cacheExec(
							*tx, "INSERT INTO next_hard_state(object_id, next_hard_state) VALUES (?, ?)",
							idoRow.ObjectId, idoRow.LastHardState,
						)

						cacheExec(
							*tx, "INSERT INTO next_ids(history_id, object_id) VALUES (?, ?)",
							idoRow.Id, idoRow.ObjectId,
						)
					}

					commitPeriodically()
					checkpoint = idoRow.Id
				}

				ht.bar.IncrBy(len(idoRows))
				return
			},
		)

		// No past hard state is available for the processed future time points, assuming pending:
		cacheExec(
			*tx, "INSERT INTO previous_hard_state(history_id, previous_hard_state) SELECT history_id, 99 FROM next_ids",
		)

		// Now they should have what they wanted:
		cacheExec(*tx, "DELETE FROM next_hard_state")
		cacheExec(*tx, "DELETE FROM next_ids")
	})

	ht.bar.SetTotal(ht.bar.Current(), true)
}

// chunkCacheTx rationale: during do operate on cache via *tx. After every completed operation call commitPeriodically()
// which periodically commits *tx and starts a new tx. (That's why tx is a **, not just a *.)
// (On non-recoverable errors the whole program exits.)
func chunkCacheTx(cache *sqlx.DB, do func(tx **sqlx.Tx, commitPeriodically func())) {
	logger := log.With("backend", "cache")
	var callsSinceLastTx int

	tx, err := cache.Beginx()
	if err != nil {
		logger.Fatalf("%+v", errors.Wrap(err, "can't begin transaction"))
	}

	do(&tx, func() { // commitPeriodically
		callsSinceLastTx++
		if callsSinceLastTx == 10000 {
			if err := tx.Commit(); err != nil {
				logger.Fatalf("%+v", errors.Wrap(err, "can't commit transaction"))
			}

			var err error

			tx, err = cache.Beginx()
			if err != nil {
				logger.Fatalf("%+v", errors.Wrap(err, "can't begin transaction"))
			}

			callsSinceLastTx = 0
		}
	})

	if err := tx.Commit(); err != nil {
		logger.Fatalf("%+v", errors.Wrap(err, "can't commit transaction"))
	}
}

// cacheGet does cache.Get(dest, query, args...). (On non-recoverable errors the whole program exits.)
func cacheGet(cache interface {
	Get(dest interface{}, query string, args ...interface{}) error
}, dest interface{}, query string, args ...interface{}) {
	if err := cache.Get(dest, query, args...); err != nil {
		log.With("backend", "cache", "query", query, "args", args).
			Fatalf("%+v", errors.Wrap(err, "can't perform query"))
	}
}

// cacheSelect does cacheTx.Select(dest, query, args...). (On non-recoverable errors the whole program exits.)
func cacheSelect(cacheTx *sqlx.Tx, dest interface{}, query string, args ...interface{}) {
	if err := cacheTx.Select(dest, query, args...); err != nil {
		log.With("backend", "cache", "query", query, "args", args).
			Fatalf("%+v", errors.Wrap(err, "can't perform query"))
	}
}

// cacheExec does cacheTx.Exec(dml, args...). On non-recoverable errors the whole program exits.
func cacheExec(cacheTx *sqlx.Tx, dml string, args ...interface{}) {
	if _, err := cacheTx.Exec(dml, args...); err != nil {
		log.With("backend", "cache", "dml", dml, "args", args).Fatalf("%+v", errors.Wrap(err, "can't perform DML"))
	}
}
