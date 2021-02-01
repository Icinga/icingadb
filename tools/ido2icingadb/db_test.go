package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"reflect"
	"testing"
)

type row struct {
	I int8
	R float32
	T string
	B []byte
}

const query = "SELECT i, r, t, b FROM test ORDER BY i"

var expected = [3]row{
	{-5, -6.25, "-7", []byte("-8")},
	{1, 2.5, "3", []byte{'4'}},
	{9, 10.125, "11", []byte("12")},
}

func TestDatabase_query(t *testing.T) {
	db := mkTestDb(t, "TestDatabase_query")
	defer db.Close()

	dB := database{conn: db}
	var actual []row

	dB.query(query, nil, func(r row) { actual = append(actual, r) })

	if len(actual) == len(expected) {
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

func TestTx_fetchAll(t *testing.T) {
	db := mkTestDb(t, "TestTx_fetchAll")
	defer db.Close()

	dB := database{conn: db}
	var actual []row

	tx := dB.begin(sql.LevelSerializable, true)
	defer tx.commit()

	tx.fetchAll(&actual, query)
	if !reflect.DeepEqual(actual, expected[:]) {
		t.Errorf("Yielded %#v, not %#v", actual, expected[:])
	}

	assertPanic(t, func() {
		tx.fetchAll(0, query)
	})

	assertPanic(t, func() {
		tx.fetchAll(new(int), query)
	})

	assertPanic(t, func() {
		tx.fetchAll(new([]int), query)
	})

	assertPanic(t, func() {
		tx.fetchAll((*[]row)(nil), query)
	})
}

func TestTx_query(t *testing.T) {
	db := mkTestDb(t, "TestTx_query")
	defer db.Close()

	dB := database{conn: db}
	var actual []row

	tx := dB.begin(sql.LevelSerializable, true)
	defer tx.commit()

	tx.query(query, nil, func(r row) { actual = append(actual, r) })

	if len(actual) == len(expected) {
		for i, v := range actual {
			if !reflect.DeepEqual(v, expected[i]) {
				t.Errorf("function got %#v, not %#v on %d. call", v, expected[i], i+1)
			}
		}
	} else {
		t.Errorf("function called %d, not 3 times", len(actual))
	}

	assertPanic(t, func() {
		tx.query(query, nil, 0)
	})

	assertPanic(t, func() {
		tx.query(query, nil, func(_, _ row) {})
	})

	assertPanic(t, func() {
		tx.query(query, nil, func(int) {})
	})
}

func TestStreamQuery(t *testing.T) {
	db := mkTestDb(t, "TestStreamQuery")
	defer db.Close()

	dB := database{conn: db}
	var actual []row

	{
		ch := make(chan row)
		go streamQuery(&dB, ch, query, nil)

		for r := range ch {
			actual = append(actual, r)
		}
	}

	if len(actual) == len(expected) {
		for i, v := range actual {
			if !reflect.DeepEqual(v, expected[i]) {
				t.Errorf("channel received %#v, not %#v on %d. time", v, expected[i], i+1)
			}
		}
	} else {
		t.Errorf("channel received %d, not 3 items", len(actual))
	}

	assertPanic(t, func() {
		streamQuery(&dB, []row(nil), query, nil)
	})

	assertPanic(t, func() {
		streamQuery(&dB, make(chan int), query, nil)
	})
}

func mkTestDb(t *testing.T, name string) *sql.DB {
	db, errOp := sql.Open("sqlite3", "file:"+name+"?mode=memory&cache=shared")
	if errOp != nil {
		t.Fatal(errOp)
	}

	db.SetMaxIdleConns(1)

	if _, errEx := db.Exec("CREATE TABLE test (i INT, r REAL, t TEXT, b BLOB)"); errEx != nil {
		t.Fatal(errEx)
	}

	_, errEx := db.Exec(
		"INSERT INTO test(i, r, t, b) VALUES (?, ?, ?, ?), (?, ?, ?, ?), (?, ?, ?, ?)",
		1, 2.5, "3", []byte{'4'},
		-5, -6.25, "-7", []byte("-8"),
		9, 10.125, "11", []byte("12"),
	)
	if errEx != nil {
		t.Fatal(errEx)
	}

	return db
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
