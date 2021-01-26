package main

import (
	"database/sql"
	"flag"
	"fmt"
	icingaSql "github.com/Icinga/go-libs/sql"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// stringValue allows to differ a string not passed via the CLI and an empty string passed via the CLI
// w/o polluting the usage instructions.
type stringValue struct {
	// value is the string passed via the CLI if any.
	value string
	// isSet tells whether the string was passed.
	isSet bool
}

var _ flag.Value = (*stringValue)(nil)

// String implements flag.Value.
func (sv *stringValue) String() string {
	return sv.value
}

// Set implements flag.Value.
func (sv *stringValue) Set(s string) error {
	sv.value = s
	sv.isSet = true
	return nil
}

// database summarizes everything regarding a particular database.
type database struct {
	// whichOne is a human-readable description what's the database for.
	whichOne string
	// cliPrefix is derived from whichOne and allows to specify multiple databases at once via the CLI.
	cliPrefix string
	// conn allows actually using the database.
	conn *sql.DB
	// stmts caches prepared SQL statements.
	stmts map[string]*sql.Stmt

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

	db.conn.SetMaxIdleConns(1)

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

		vOnRow.Call([]reflect.Value{vRes.Index(0)})
	}
}

// exec prepares and executes query with args on db.
func (db *database) exec(query string, args ...interface{}) {
	stmt, ok := db.stmts[query]
	if !ok {
		var errPP error
		stmt, errPP = db.conn.Prepare(query)
		assert(errPP, "Couldn't prepare SQL statement", log.Fields{"backend": db.whichOne, "statement": query})

		db.stmts[query] = stmt
	}

	_, errEx := stmt.Exec(args...)
	assert(
		errEx,
		"Couldn't execute prepared SQL statement",
		log.Fields{"backend": db.whichOne, "statement": query, "args": args},
	)
}

// newDb creates a new database.
func newDb(whichOne string) database {
	cliPrefix := strings.ToLower(strings.ReplaceAll(whichOne, " ", "")) + "-"

	return database{
		whichOne, cliPrefix, nil, map[string]*sql.Stmt{},
		flag.String(cliPrefix+"host", "", "HOST/SOCKET"),
		flag.Int(cliPrefix+"port", 0, "PORT"),
		flag.String(cliPrefix+"db", "", "DATABASE"),
		flag.String(cliPrefix+"user", "", "USER"),
		flag.String(cliPrefix+"pass", "", "PASSWORD"),
	}
}

var ido = newDb("IDO")
var icingaDb = newDb("Icinga DB")
var bulk = flag.Int("bulk", 200, "FACTOR")

var icingaEnv, icingaEndpoint stringValue
var envId, endpointId []byte

func main() {
	flag.Var(&icingaEnv, "icinga-env", "ENVIRONMENT")
	flag.Var(&icingaEndpoint, "icinga-endpoint", "ENDPOINT")
	flag.Parse()

	ido.validate()
	icingaDb.validate()

	if !icingaEnv.isSet {
		fmt.Fprintln(os.Stderr, "-icinga-env missing")
		flag.Usage()
		os.Exit(2)
	}

	if !icingaEndpoint.isSet {
		fmt.Fprintln(os.Stderr, "-icinga-endpoint missing")
		flag.Usage()
		os.Exit(2)
	}

	envId = hashStr(icingaEnv.value)
	endpointId = hashAny([2]string{icingaEnv.value, icingaEndpoint.value})

	ido.connect()
	icingaDb.connect()

	syncStates()
}

// assert logs message with fields and err and terminates the program if err is not nil.
func assert(err error, message string, fields log.Fields) {
	if err != nil {
		log.WithFields(fields).WithFields(log.Fields{"error": err.Error()}).Fatal(message)
	}
}
