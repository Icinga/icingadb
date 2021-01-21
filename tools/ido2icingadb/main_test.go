package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"reflect"
	"testing"
)

func TestDatabase_query(t *testing.T) {
	type row struct {
		I int8
		R float32
		T string
		B []byte
	}

	db, errOp := sql.Open("sqlite3", "file::memory:?cache=shared")
	if errOp != nil {
		t.Fatal(errOp)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, errEx := db.Exec("CREATE TABLE test (i INT, r REAL, t TEXT, b BLOB)"); errEx != nil {
		t.Fatal(errEx)
	}

	{
		_, errEx := db.Exec(
			"INSERT INTO test(i, r, t, b) VALUES (?, ?, ?, ?), (?, ?, ?, ?), (?, ?, ?, ?)",
			1, 2.5, "3", []byte{'4'},
			-5, -6.25, "-7", []byte("-8"),
			9, 10.125, "11", []byte("12"),
		)
		if errEx != nil {
			t.Fatal(errEx)
		}
	}

	dB := database{conn: db}
	var actual []row
	const query = "SELECT i, r, t, b FROM test ORDER BY i"

	dB.query(query, nil, func(r row) { actual = append(actual, r) })

	if len(actual) == 3 {
		expected := []row{
			{-5, -6.25, "-7", []byte("-8")},
			{1, 2.5, "3", []byte{'4'}},
			{9, 10.125, "11", []byte("12")},
		}
		for i, v := range actual {
			if !reflect.DeepEqual(v, expected[i]) {
				t.Errorf("function got %#v, not %#v on %d. call", v, expected[i], i+1)
			}
		}
	} else {
		t.Errorf("function called %d, not 3 times", len(actual))
	}

	assertPanic(t, func() {
		dB.query(query, nil, 0)
	})

	assertPanic(t, func() {
		dB.query(query, nil, func(_, _ row) {})
	})

	assertPanic(t, func() {
		dB.query(query, nil, func(int) {})
	})
}

func assertPanic(t *testing.T, f func()) {
	t.Helper()

	defer func() {
		t.Helper()

		if recover() == nil {
			t.Error("Didn't panic")
		}
	}()

	f()
}
