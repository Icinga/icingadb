package main

import (
	"bytes"
	"database/sql"
	"testing"
)

func TestGetLastSyncedId(t *testing.T) {
	db, errOp := sql.Open("sqlite3", "file:TestGetLastSyncedId?mode=memory&cache=shared")
	if errOp != nil {
		t.Fatal(errOp)
	}

	db.SetMaxIdleConns(1)

	if _, errEx := db.Exec("CREATE TABLE icinga_statehistory (statehistory_id INT)"); errEx != nil {
		t.Fatal(errEx)
	}

	if _, errEx := db.Exec("CREATE TABLE state_history (id BLOB)"); errEx != nil {
		t.Fatal(errEx)
	}

	ido.conn = db
	icingaDb.conn = db

	total, done, lsi := getProgress(stateHistory, "icinga_statehistory", "statehistory_id", "state_history")
	if total != 0 || done != 0 || lsi != 0 {
		t.Error("getProgress() must return 0,0,0 if the IDO table is empty")
	}

	for start := 1; start < 3; start++ {
		for evenOdd := 0; evenOdd < 2; evenOdd++ {
			end := start + 10 + evenOdd
			for lastSynced := start; lastSynced <= end; lastSynced++ {
				if _, errEx := db.Exec("DELETE FROM icinga_statehistory"); errEx != nil {
					t.Fatal(errEx)
				}

				if _, errEx := db.Exec("DELETE FROM state_history"); errEx != nil {
					t.Fatal(errEx)
				}

				for i := start; i <= end; i++ {
					_, errEx := db.Exec("INSERT INTO icinga_statehistory(statehistory_id) VALUES (?)", i)
					if errEx != nil {
						t.Fatal(errEx)
					}
				}

				for i := start; i <= lastSynced; i++ {
					_, errEx := db.Exec(
						"INSERT INTO state_history(id) VALUES (?)",
						mkDeterministicUuid(stateHistory, uint64(i)),
					)
					if errEx != nil {
						t.Fatal(errEx)
					}
				}

				total, done, lsi := getProgress(stateHistory, "icinga_statehistory", "statehistory_id", "state_history")

				if total != int64(evenOdd)+11 {
					t.Error("getProgress()'s total is wrong")
				}

				if done > int64(lastSynced-start) {
					t.Error("getProgress()'s done is too high")
				}

				if lsi > int64(lastSynced) {
					t.Error("getProgress()'s lastSyncedId is too high")
				}
			}
		}
	}
}

func TestMkDeterministicUuid(t *testing.T) {
	if bytes.Compare(
		mkDeterministicUuid(stateHistory, 0x0102030405060708),
		[]byte{'I', 'D', 'O', 's', 'h', 0, 0x40, 1, 0x80, 2, 3, 4, 5, 6, 7, 8},
	) != 0 {
		t.Error("got wrong UUID from mkDeterministicUuid(stateHistory, 0x0102030405060708)")
	}
}
