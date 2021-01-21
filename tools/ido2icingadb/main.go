package main

import (
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
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
		assert(errOp, "Couldn't connect to %s database", db.whichOne)
	}

	db.conn.SetMaxIdleConns(1)

	assert(db.conn.Ping(), "Couldn't connect to %s database", db.whichOne)
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

func main() {
	ido := newDb("IDO")
	icingaDb := newDb("Icinga DB")

	flag.Parse()

	ido.validate()
	icingaDb.validate()

	ido.connect()
	icingaDb.connect()
}

// assert logs messagef with messagea and terminates the program if err is not nil.
func assert(err error, messagef string, messagea ...interface{}) {
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Fatalf(messagef, messagea...)
	}
}
