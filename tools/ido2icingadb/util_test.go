package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"
)

func TestGetLastSyncedId(t *testing.T) {
	testGetLastSyncedIdWithDbs(t, func() {}, func(idoTx tx) {
		total, done, lsi := getProgress(
			idoTx, "icinga_statehistory", "statehistory_id", "state_history", "id",
			func(idoId uint64) interface{} { return mkDeterministicUuid(stateHistory, idoId) },
		)
		if total != 0 || done != 0 || lsi != 0 {
			t.Error("getProgress() must return 0,0,0 if the IDO table is empty")
		}
	})

	for start := 1; start < 3; start++ {
		for evenOdd := 0; evenOdd < 2; evenOdd++ {
			end := start + 10 + evenOdd
			for lastSynced := start; lastSynced <= end; lastSynced++ {
				testGetLastSyncedIdWithDbs(
					t,
					func() {
						for i := start; i <= end; i++ {
							_, errEx := ido.conn.Exec(
								"INSERT INTO icinga_statehistory(statehistory_id) VALUES (?)", i*10,
							)
							if errEx != nil {
								t.Fatal(errEx)
							}
						}

						for i := start; i <= lastSynced; i++ {
							_, errEx := icingaDb.conn.Exec(
								"INSERT INTO state_history(id) VALUES (?)",
								mkDeterministicUuid(stateHistory, uint64(i*10)),
							)
							if errEx != nil {
								t.Fatal(errEx)
							}
						}
					},
					func(idoTx tx) {
						total, done, lsi := getProgress(
							idoTx, "icinga_statehistory", "statehistory_id", "state_history", "id",
							func(idoId uint64) interface{} { return mkDeterministicUuid(stateHistory, idoId) },
						)

						if total != int64(evenOdd)+11 {
							t.Error("getProgress()'s total is wrong")
						}

						if done != int64(lastSynced-start+1) {
							t.Error("getProgress()'s done is wrong")
						}

						if lsi != int64(lastSynced*10) {
							t.Error("getProgress()'s lastSyncedId is wrong")
						}
					},
				)
			}
		}
	}
}

var testGetLastSyncedIdDbNr uint64

func testGetLastSyncedIdWithDbs(t *testing.T, prepare func(), testCase func(idoTx tx)) {
	t.Helper()

	{
		db, errOp := sql.Open("sqlite3", fmt.Sprintf(
			"file:TestGetLastSyncedId%d?mode=memory&cache=shared",
			atomic.AddUint64(&testGetLastSyncedIdDbNr, 1),
		))
		if errOp != nil {
			t.Fatal(errOp)
		}

		defer db.Close()

		db.SetMaxIdleConns(1)

		if _, errEx := db.Exec("CREATE TABLE icinga_statehistory (statehistory_id INT)"); errEx != nil {
			t.Fatal(errEx)
		}

		ido.conn = db
	}

	{
		db, errOp := sql.Open("sqlite3", fmt.Sprintf(
			"file:TestGetLastSyncedId%d?mode=memory&cache=shared",
			atomic.AddUint64(&testGetLastSyncedIdDbNr, 1),
		))
		if errOp != nil {
			t.Fatal(errOp)
		}

		defer db.Close()

		db.SetMaxIdleConns(1)

		if _, errEx := db.Exec("CREATE TABLE state_history (id BLOB)"); errEx != nil {
			t.Fatal(errEx)
		}

		icingaDb.conn = db
	}

	prepare()

	idoTx := ido.begin(sql.LevelRepeatableRead, true)
	defer idoTx.tx.Rollback()

	testCase(idoTx)
}

func TestMkDeterministicUuid(t *testing.T) {
	if bytes.Compare(
		mkDeterministicUuid(stateHistory, 0x0102030405060708),
		[]byte{'I', 'D', 'O', 's', 'h', 0, 0x40, 1, 0x80, 2, 3, 4, 5, 6, 7, 8},
	) != 0 {
		t.Error("got wrong UUID from mkDeterministicUuid(stateHistory, 0x0102030405060708)")
	}
}
