package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	icingaSql "github.com/Icinga/go-libs/sql"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// database summarizes everything regarding a particular database.
type database struct {
	// whichOne is a human-readable description what's the database for.
	whichOne string
	// cliPrefix is derived from whichOne and allows to specify multiple databases at once via the CLI.
	cliPrefix string
	// conn allows actually using the database.
	conn *sql.DB

	// host is either the RDBMS' host or *nix socket.
	host *string
	// port is the RDBMS' port.
	port *int
	// db is the database name.
	db *string
	// user is a user allowed to access the database.
	user *string
	// pass is the above user's password.
	pass *string
}

// validate complains if db was not specified via the CLI.
func (db *database) validate() {
	if *db.db == "" {
		fmt.Fprintf(os.Stderr, "-%sdb missing\n", db.cliPrefix)
		flag.Usage()
		os.Exit(2)
	}
}

// connect connects to db an initializes conn.
func (db *database) connect() {
	{
		var dsn string
		var errOp error

		if *db.user != "" {
			dsn += *db.user

			if *db.pass != "" {
				dsn += ":" + *db.pass
			}

			dsn += "@"
		}

		if *db.host != "" {
			if filepath.IsAbs(*db.host) {
				dsn += fmt.Sprintf("unix(%s)", *db.host)
			} else {
				dsn += "tcp(" + *db.host

				if *db.port != 0 {
					dsn += fmt.Sprintf(":%d", *db.port)
				}

				dsn += ")"
			}
		}

		dsn += "/" + *db.db + "?innodb_strict_mode=1&sql_mode='STRICT_ALL_TABLES,NO_ZERO_IN_DATE," +
			"NO_ZERO_DATE,NO_ENGINE_SUBSTITUTION,PIPES_AS_CONCAT,ANSI_QUOTES,ERROR_FOR_DIVISION_BY_ZERO'"

		db.conn, errOp = sql.Open("mysql", dsn)
		assert(errOp, "Couldn't connect to database", log.Fields{"backend": db.whichOne})
	}

	db.conn.SetMaxIdleConns(16)

	assert(db.conn.Ping(), "Couldn't connect to database", log.Fields{"backend": db.whichOne})
}

// query performs query with args on db and calls onRow for each row.
func (db *database) query(query string, args []interface{}, onRow interface{}) {
	vOnRow := reflect.ValueOf(onRow)
	tOnRow := vOnRow.Type()

	if tOnRow.Kind() != reflect.Func {
		panic("onRow must be a function")
	}

	if tOnRow.NumIn() != 1 {
		panic("onRow must take exactly one argument")
	}

	tOnRowArg := tOnRow.In(0)
	if tOnRowArg.Kind() != reflect.Struct {
		panic("onRow must take a struct")
	}

	rows, errQr := db.conn.Query(query, args...)
	assert(errQr, "Couldn't perform query", log.Fields{"backend": db.whichOne, "query": query, "args": args})
	defer rows.Close()

	iOnRowArg := reflect.Zero(tOnRowArg).Interface()
	i := 0

	for {
		i++

		res, errFR := icingaSql.FetchRowsAsStructSlice(rows, iOnRowArg, 1)
		assert(
			errFR,
			"Couldn't fetch query result",
			log.Fields{"backend": db.whichOne, "query": query, "args": args, "row": i},
		)

		vRes := reflect.ValueOf(res)
		if vRes.Len() < 1 {
			break
		}

		ret := vOnRow.Call([]reflect.Value{vRes.Index(0)})
		if len(ret) > 0 && ret[0].Kind() == reflect.Bool && !ret[0].Bool() {
			break
		}
	}
}

// exec executes query with args on db.
func (db *database) exec(query string, args ...interface{}) {
	_, errEx := db.conn.Exec(query, args...)
	assert(
		errEx,
		"Couldn't execute SQL statement",
		log.Fields{"backend": db.whichOne, "statement": query, "args": args},
	)
}

// begin wraps db.conn.BeginTx for error handling.
func (db *database) begin(isolation sql.IsolationLevel, readOnly bool) tx {
	t, errBg := db.conn.BeginTx(context.Background(), &sql.TxOptions{Isolation: isolation, ReadOnly: readOnly})
	assert(errBg, "Couldn't begin transaction", log.Fields{"backend": db.whichOne})
	return tx{db, t}
}

// newDb creates a new database.
func newDb(whichOne string) database {
	cliPrefix := strings.ToLower(strings.ReplaceAll(whichOne, " ", "")) + "-"

	return database{
		whichOne, cliPrefix, nil,
		flag.String(cliPrefix+"host", "", "HOST/SOCKET"),
		flag.Int(cliPrefix+"port", 0, "PORT"),
		flag.String(cliPrefix+"db", "", "DATABASE"),
		flag.String(cliPrefix+"user", "", "USER"),
		flag.String(cliPrefix+"pass", "", "PASSWORD"),
	}
}

// tx wraps sql.Tx for error handling.
type tx struct {
	db *database
	tx *sql.Tx
}

// fetchAll performs query with args and stores the result into *res.
func (t tx) fetchAll(res interface{}, query string, args ...interface{}) {
	vRes := reflect.ValueOf(res)
	tRes := vRes.Type()

	if tRes.Kind() != reflect.Ptr {
		panic("res must be a pointer")
	}

	tResElem := tRes.Elem()
	if tResElem.Kind() != reflect.Slice {
		panic("res must point to a slice")
	}

	tResElemElem := tResElem.Elem()
	if tResElemElem.Kind() != reflect.Struct {
		panic("res must point to a slice of structs")
	}

	rows, errQr := t.tx.Query(query, args...)
	assert(errQr, "Couldn't perform query", log.Fields{"backend": t.db.whichOne, "query": query, "args": args})
	defer rows.Close()

	iRes, errFR := icingaSql.FetchRowsAsStructSlice(rows, reflect.Zero(tResElemElem).Interface(), -1)
	assert(
		errFR, "Couldn't fetch query result",
		log.Fields{"backend": t.db.whichOne, "query": query, "args": args},
	)

	vRes.Elem().Set(reflect.ValueOf(iRes))
}

// query performs query with args on t and calls onRow for each row.
func (t tx) query(query string, args []interface{}, onRow interface{}) {
	vOnRow := reflect.ValueOf(onRow)
	tOnRow := vOnRow.Type()

	if tOnRow.Kind() != reflect.Func {
		panic("onRow must be a function")
	}

	if tOnRow.NumIn() != 1 {
		panic("onRow must take exactly one argument")
	}

	tOnRowArg := tOnRow.In(0)
	if tOnRowArg.Kind() != reflect.Struct {
		panic("onRow must take a struct")
	}

	rows, errQr := t.db.conn.Query(query, args...)
	assert(errQr, "Couldn't perform query", log.Fields{"backend": t.db.whichOne, "query": query, "args": args})
	defer rows.Close()

	iOnRowArg := reflect.Zero(tOnRowArg).Interface()
	i := 0

	for {
		i++

		res, errFR := icingaSql.FetchRowsAsStructSlice(rows, iOnRowArg, 1)
		assert(
			errFR,
			"Couldn't fetch query result",
			log.Fields{"backend": t.db.whichOne, "query": query, "args": args, "row": i},
		)

		vRes := reflect.ValueOf(res)
		if vRes.Len() < 1 {
			break
		}

		ret := vOnRow.Call([]reflect.Value{vRes.Index(0)})
		if len(ret) > 0 && ret[0].Kind() == reflect.Bool && !ret[0].Bool() {
			break
		}
	}
}

func (t tx) exec(query string, args ...interface{}) int64 {
	res, errEx := t.tx.Exec(query, args...)
	assert(
		errEx, "Couldn't execute SQL statement",
		log.Fields{"backend": t.db.whichOne, "statement": query, "args": args},
	)

	amount, errRA := res.RowsAffected()
	assert(
		errRA, "Couldn't figure out amount of rows affected by SQL statement",
		log.Fields{"backend": t.db.whichOne, "statement": query, "args": args},
	)

	return amount
}

func (t tx) commit() {
	assert(t.tx.Commit(), "Couldn't commit transaction", log.Fields{"backend": t.db.whichOne})
}

// queryable abstracts *database and tx.
type queryable interface {
	query(query string, args []interface{}, onRow interface{})
}

// streamQuery performs query with args on source and sends each row to dest.
func streamQuery(source queryable, dest interface{}, query string, args ...interface{}) {
	vDest := reflect.ValueOf(dest)
	tDest := vDest.Type()

	if tDest.Kind() != reflect.Chan {
		panic("dest must be a channel")
	}

	tDestElem := tDest.Elem()
	if tDestElem.Kind() != reflect.Struct {
		panic("dest must be a channel of structs")
	}

	defer vDest.Close()

	source.query(query, args, reflect.MakeFunc(
		reflect.FuncOf([]reflect.Type{tDestElem}, nil, false),
		func(args []reflect.Value) []reflect.Value {
			vDest.Send(args[0])
			return nil
		},
	).Interface())
}
