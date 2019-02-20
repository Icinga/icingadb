package icingadb_connection

import (
	"container/list"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"git.icinga.com/icingadb/icingadb/benchmark"
	"github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	oldlog "log"
	"io/ioutil"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type dbTypeBridge interface {
	sql.Scanner
	Result() interface{}
}

type dbIntBridge struct {
	result interface{}
}

func (d *dbIntBridge) Scan(src interface{}) (err error) {
	baseScanner := sql.NullInt64{}
	err = baseScanner.Scan(src)

	if err == nil {
		if baseScanner.Valid {
			d.result = baseScanner.Int64
		} else {
			d.result = nil
		}
	}

	return
}

func (d *dbIntBridge) Result() interface{} {
	return d.result
}

type dbFloatBridge struct {
	result interface{}
}

func (d *dbFloatBridge) Scan(src interface{}) (err error) {
	baseScanner := sql.NullFloat64{}
	err = baseScanner.Scan(src)

	if err == nil {
		if baseScanner.Valid {
			d.result = baseScanner.Float64
		} else {
			d.result = nil
		}
	}

	return
}

func (d *dbFloatBridge) Result() interface{} {
	return d.result
}

type dbStringBridge struct {
	result interface{}
}

func (d *dbStringBridge) Scan(src interface{}) (err error) {
	baseScanner := sql.NullString{}
	err = baseScanner.Scan(src)

	if err == nil {
		if baseScanner.Valid {
			d.result = baseScanner.String
		} else {
			d.result = nil
		}
	}

	return
}

func (d *dbStringBridge) Result() interface{} {
	return d.result
}

type dbBytesBridge struct {
	result interface{}
}

func (d *dbBytesBridge) Scan(src interface{}) (err error) {
	baseScanner := sql.NullString{}
	err = baseScanner.Scan(src)

	if err == nil {
		if baseScanner.Valid {
			d.result = []byte(baseScanner.String)
		} else {
			d.result = nil
		}
	}

	return
}

func (d *dbBytesBridge) Result() interface{} {
	return d.result
}

var dbTypeBridgeFactories = map[string]func() dbTypeBridge{
	// MySQL
	"TINYINT": func() dbTypeBridge {
		return &dbIntBridge{}
	},
	"SMALLINT": func() dbTypeBridge {
		return &dbIntBridge{}
	},
	"INT": func() dbTypeBridge {
		return &dbIntBridge{}
	},
	"BIGINT": func() dbTypeBridge {
		return &dbIntBridge{}
	},
	"FLOAT": func() dbTypeBridge {
		return &dbFloatBridge{}
	},
	"CHAR": func() dbTypeBridge {
		return &dbStringBridge{}
	},
	"VARCHAR": func() dbTypeBridge {
		return &dbStringBridge{}
	},
	"ENUM": func() dbTypeBridge {
		return &dbStringBridge{}
	},
	"BINARY": func() dbTypeBridge {
		return &dbBytesBridge{}
	},

	// SQLite
	"INTEGER": func() dbTypeBridge {
		return &dbIntBridge{}
	},
	"REAL": func() dbTypeBridge {
		return &dbFloatBridge{}
	},
	"TEXT": func() dbTypeBridge {
		return &dbStringBridge{}
	},
	"BLOB": func() dbTypeBridge {
		return &dbBytesBridge{}
	},
	// SELECT 1 FROM ...
	"": func() dbTypeBridge {
		return &dbIntBridge{}
	},
}

type dbBrokenBridge struct {
	typ string
}

func (d *dbBrokenBridge) Scan(src interface{}) error {
	types := make([]string, len(dbTypeBridgeFactories))
	typeIdx := 0

	for typ := range dbTypeBridgeFactories {
		types[typeIdx] = typ
		typeIdx++
	}

	sort.Strings(types)

	return errors.New(fmt.Sprintf("bad column type %s, expected one of %s", d.typ, strings.Join(types, ", ")))
}

func (d *dbBrokenBridge) Result() interface{} {
	return nil
}

var prettyPrintedSqlReplacer = strings.NewReplacer("\n", " ", "\t", "")

type prettyPrintedSql struct {
	sql string
}

// String implements and interface from Stringer
func (p prettyPrintedSql) String() string {
	return strings.TrimSpace(prettyPrintedSqlReplacer.Replace(p.sql))
}

// MarshalText implements an interface from TextMarshaler
func (p prettyPrintedSql) MarshalText() (text []byte, err error) {
	return []byte(p.String()), nil
}

type prettyPrintedArgs struct {
	args []interface{}
}

func (p *prettyPrintedArgs) String() string {
	res := "["

	for _, v := range p.args {
		if byteArray, isByteArray := v.([]byte); isByteArray {
			res = fmt.Sprintf("%s hex.DecodeString(\"%s\"),", res, hex.EncodeToString(byteArray))
		} else {
			res = fmt.Sprintf("%s %#v,", res, v)
		}
	}

	return res + " ]"
}

// MarshalText implements an interface from TextMarshaler
func (p prettyPrintedArgs) MarshalText() (text []byte, err error) {
	return []byte(p.String()), nil
}

type prettyPrintedRowsAffected struct {
	result sql.Result
}

// String implements and interface from Stringer
func (d prettyPrintedRowsAffected) String() string {
	if d.result != nil {
		rows, errRA := d.result.RowsAffected()
		if errRA == nil {
			return strconv.FormatInt(rows, 10)
		}
	}

	return "N/A"
}

// MarshalText implements an interface from TextMarshaler
func (d prettyPrintedRowsAffected) MarshalText() (text []byte, err error) {
	return []byte(d.String()), nil
}

// Either a connection or a transaction
type DbClient interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

// Database wrapper including helper functions
type DBWrapper struct {
	Db                    *sql.DB
	ConnectedAtomic       *uint32 //uint32 to be able to use atomic operations
	ConnectionUpCondition *sync.Cond
	ConnectionLostCounter int
}

func (dbw *DBWrapper) IsConnected() bool {
	return *dbw.ConnectedAtomic != 0
}

func (dbw *DBWrapper) CompareAndSetConnected(connected bool) (swapped bool) {
	if connected {
		return atomic.CompareAndSwapUint32(dbw.ConnectedAtomic, 0, 1)
	} else {
		return atomic.CompareAndSwapUint32(dbw.ConnectedAtomic, 1, 0)
	}
}

// mkMysql creates a new MySQL client.
func mkMysql(dbType string, dbDsn string) (*sql.DB, error) {
	log.Info("Connecting to MySQL")

	sep := "?"

	if dbDsn == "" {
		dbDsn = "/"
	} else {
		dsnParts := strings.Split(dbDsn, "/")
		if strings.Contains(dsnParts[len(dsnParts)-1], "?") {
			sep = "&"
		}
	}

	dbDsn = dbDsn + sep +
		"innodb_strict_mode=1&sql_mode='STRICT_ALL_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,NO_ENGINE_SUBSTITUTION,PIPES_AS_CONCAT,ANSI_QUOTES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER'"

	db, errConn := sql.Open(dbType, dbDsn)
	if errConn != nil {
		return nil, errConn
	}

	mysql.SetLogger(oldlog.New(ioutil.Discard, "", 0))

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(0)

	return db, nil
}

func NewDBWrapper(dbType string, dbDsn string) (*DBWrapper, error) {
	db, err := mkMysql(dbType, dbDsn)

	if err != nil {
		return nil, err
	}

	dbw := DBWrapper{Db: db, ConnectedAtomic: new(uint32)}
	dbw.ConnectionUpCondition = sync.NewCond(&sync.Mutex{})

	go func() {
		for {
			dbw.checkConnection(true)
			time.Sleep(dbw.getConnectionCheckInterval())
		}
	}()

	return &dbw, nil
}

func (dbw *DBWrapper) getConnectionCheckInterval() time.Duration {
	if !dbw.IsConnected() {
		if dbw.ConnectionLostCounter < 4 {
			return 5 * time.Second
		} else if dbw.ConnectionLostCounter < 8 {
			return 10 * time.Second
		} else if dbw.ConnectionLostCounter < 11 {
			return 30 * time.Second
		} else if dbw.ConnectionLostCounter < 14 {
			return 60 * time.Second
		} else {
			log.Fatal("Could not connect to SQL for over 5 minutes. Shutting down...")
		}
	}

	return 15 * time.Second
}

func (dbw *DBWrapper) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return dbw.Db.Query(query, args...)
}

func (dbw *DBWrapper) checkConnection(isTicker bool) bool {
	err := dbw.Db.Ping()
	if err != nil {
		if dbw.CompareAndSetConnected(false) {
			log.WithFields(log.Fields{
				"context": "sql",
				"error":   err,
			}).Error("SQL connection lost. Trying to reconnect")
		} else if isTicker {
			dbw.ConnectionLostCounter++

			log.WithFields(log.Fields{
				"context": "sql",
				"error":   err,
			}).Debugf("SQL connection lost. Trying again in %s", dbw.getConnectionCheckInterval())
		}

		return false
	} else {
		if dbw.CompareAndSetConnected(true) {
			log.Info("SQL connection established")
			dbw.ConnectionLostCounter = 0
			dbw.ConnectionUpCondition.Broadcast()
		}

		return true
	}
}

func (dbw *DBWrapper) WaitForConnection() {
	dbw.ConnectionUpCondition.L.Lock()
	dbw.ConnectionUpCondition.Wait()
	dbw.ConnectionUpCondition.L.Unlock()
}

type MysqlConnectionError struct {
	err string
}

func (e MysqlConnectionError) Error() string {
	return e.err
}

// SqlTransaction executes the given function inside a transaction.
func (dbw DBWrapper) SqlTransaction(concurrencySafety bool, retryOnConnectionFailure bool, f func(*sql.Tx) error) error {
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		benchmarc := benchmark.NewBenchmark()
		errTx := dbw.sqlTryTransaction(f, concurrencySafety, false)
		benchmarc.Stop()

		//DbIoSeconds.WithLabelValues("mysql", "transaction").Observe(benchmarc.Seconds())

		log.WithFields(log.Fields{
			"context":   "sql",
			"benchmark": benchmarc,
		}).Debug("Executed transaction")

		if errTx != nil {
			//TODO: Do this only for concurrencySafety = true, once we figure out the serialization errors.
			if isSerializationFailure(errTx) {
				log.WithFields(log.Fields{
					"context": "sql",
					"error":   errTx,
				}).Debug("Repeating transaction")
				continue
			}

			if !dbw.checkConnection(false) {
				if retryOnConnectionFailure {
					continue
				} else {
					return MysqlConnectionError{"Transaction failed duo to a connection error"}
				}
			}

			log.WithFields(log.Fields{
				"context": "sql",
				"error":   errTx,
			}).Warn("SQL error occurred")
		}

		return errTx
	}
}

// SqlTransaction executes the given function inside a transaction.
func (dbw *DBWrapper) SqlTransactionQuiet(concurrencySafety bool, retryOnConnectionFailure bool, f func(*sql.Tx) error) error {
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		errTx := dbw.sqlTryTransaction(f, concurrencySafety, true)
		if errTx != nil {
			//TODO: Do this only for concurrencySafety = true, once we figure out the serialization errors.
			if isSerializationFailure(errTx) {
				continue
			}

			if !dbw.checkConnection(false) {
				if retryOnConnectionFailure {
					continue
				} else {
					return MysqlConnectionError{"Transaction failed duo to a connection error"}
				}
			}

			// We still log errors
			log.WithFields(log.Fields{
				"context": "sql",
				"error":   errTx,
			}).Warn("SQL error occurred")
		}
		return errTx
	}
}

// Executes the given function inside a transaction
func (dbw *DBWrapper) sqlTryTransaction(f func(*sql.Tx) error, concurrencySafety bool, quiet bool) error {
	tx, errBegin := dbw.sqlBegin(concurrencySafety, quiet)
	if errBegin != nil {
		return errBegin
	}

	errTx := f(tx)
	if errTx != nil {
		dbw.sqlRollback(tx, quiet)
		return errTx
	}

	return dbw.sqlCommit(tx, quiet)
}

// Returns whether the given error signals serialization failure
// https://dev.mysql.com/doc/refman/5.5/en/error-messages-server.html#error_er_lock_deadlock
func isSerializationFailure(e error) bool {
	switch err := e.(type) {
	case *mysql.MySQLError:
		switch err.Number {
		// Those are the error numbers for serialization failures, upon which we retry
		case 1205, 1213:
			return true
		}
	}

	return false
}

// Wrapper around Db.BeginTx() for auto-logging
func (dbw *DBWrapper) sqlBegin(concurrencySafety bool, quiet bool) (*sql.Tx, error) {
	var isoLvl sql.IsolationLevel
	if concurrencySafety {
		isoLvl = sql.LevelSerializable
	} else {
		isoLvl = sql.LevelReadCommitted
	}

	var err error
	var tx *sql.Tx
	if quiet {
		tx, err = dbw.Db.BeginTx(context.Background(), &sql.TxOptions{Isolation: isoLvl})
	} else {
		benchmarc := benchmark.NewBenchmark()
		tx, err = dbw.Db.BeginTx(context.Background(), &sql.TxOptions{Isolation: isoLvl})
		benchmarc.Stop()

		//DbIoSeconds.WithLabelValues("mysql", "begin").Observe(benchmarc.Seconds())

		log.WithFields(log.Fields{
			"context":   "sql",
			"benchmark": benchmarc,
		}).Debug("BEGIN transaction")
	}

	return tx, err
}

// Wrapper around tx.Commit() for auto-logging
func (dbw *DBWrapper) sqlCommit(tx *sql.Tx, quiet bool) error {
	var err error
	if quiet {
		err = tx.Commit()
	} else {
		benchmarc := benchmark.NewBenchmark()
		err = tx.Commit()
		benchmarc.Stop()

		//DbIoSeconds.WithLabelValues("mysql", "commit").Observe(benchmarc.Seconds())

		log.WithFields(log.Fields{
			"context":   "sql",
			"benchmark": benchmarc,
		}).Debug("COMMIT transaction")
	}

	return err
}

// Wrapper around tx.Rollback() for auto-logging
func (dbw *DBWrapper) sqlRollback(tx *sql.Tx, quiet bool) error {
	var err error
	if !quiet {
		benchmarc := benchmark.NewBenchmark()
		err = tx.Rollback()
		benchmarc.Stop()

		//DbIoSeconds.WithLabelValues("mysql", "rollback").Observe(benchmarc.Seconds())

		log.WithFields(log.Fields{
			"context":   "sql",
			"benchmark": benchmarc,
		}).Debug("ROLLBACK transaction")
	} else {
		err = tx.Rollback()
	}

	return err
}

// Wrapper around Db.SqlQuery() for auto-logging
func (dbw *DBWrapper) SqlFetchAll(db DbClient, queryDescription string, query string, args ...interface{}) ([][]interface{}, error) {
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		res, err := sqlTryFetchAll(db, queryDescription, query, args...)

		if err != nil {
			if _, isDb := db.(*sql.DB); isDb {
				if !dbw.checkConnection(false) {
					continue
				}
			}
		}

		return res, err
	}
}

func sqlTryFetchAll(db DbClient, queryDescription string, query string, args ...interface{}) ([][]interface{}, error) {
	benchmarc := benchmark.NewBenchmark()
	rows, errQuery := db.Query(query, args...)
	benchmarc.Stop()

	//DbIoSeconds.WithLabelValues("mysql", queryDescription).Observe(benchmarc.Seconds())

	rowsCount := 0

	defer func() {
		log.WithFields(log.Fields{
			"context":       "sql",
			"benchmark":     benchmarc,
			"query":         prettyPrintedSql{query},
			"args":          prettyPrintedArgs{args},
			"affected_Rows": rowsCount,
		}).Debug("Finished FetchAll")
	}()

	if errQuery != nil {
		return [][]interface{}{}, errQuery
	}

	defer rows.Close()

	columnTypes, errCT := rows.ColumnTypes()
	if errCT != nil {
		return [][]interface{}{}, errCT
	}

	colsPerRow := len(columnTypes)
	buf := list.New()
	bridges := make([]dbTypeBridge, colsPerRow)
	scanDest := make([]interface{}, colsPerRow)

	for i, columnType := range columnTypes {
		typ := columnType.DatabaseTypeName()
		factory, hasFactory := dbTypeBridgeFactories[typ]
		if hasFactory {
			bridges[i] = factory()
		} else {
			bridges[i] = &dbBrokenBridge{typ: typ}
		}

		scanDest[i] = bridges[i]
	}

	for {
		if rows.Next() {
			if errScan := rows.Scan(scanDest...); errScan != nil {
				return [][]interface{}{}, errScan
			}

			row := make([]interface{}, colsPerRow)

			for i, bridge := range bridges {
				row[i] = bridge.Result()
			}

			buf.PushBack(row)
		} else if errNx := rows.Err(); errNx == nil {
			break
		} else {
			return nil, errNx
		}
	}

	res := make([][]interface{}, buf.Len())

	for current, i := buf.Front(), 0; current != nil; current = current.Next() {
		res[i] = current.Value.([]interface{})
		i++
	}

	rowsCount = len(res)

	return res, nil
}

// No logging, no benchmarking
func (dbw *DBWrapper) SqlFetchAllQuiet(db DbClient, queryDescription string, query string, args ...interface{}) ([][]interface{}, error) {
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		res, err := sqlTryFetchAllQuiet(db, queryDescription, query, args...)

		if err != nil {
			if _, isDb := db.(*sql.DB); isDb {
				if !dbw.checkConnection(false) {
					continue
				}
			}
		}

		return res, err
	}
}

func sqlTryFetchAllQuiet(db DbClient, queryDescription string, query string, args ...interface{}) ([][]interface{}, error) {
	rows, errQuery := db.Query(query, args...)

	if errQuery != nil {
		return [][]interface{}{}, errQuery
	}

	defer rows.Close()

	columnTypes, errCT := rows.ColumnTypes()
	if errCT != nil {
		return [][]interface{}{}, errCT
	}

	colsPerRow := len(columnTypes)
	buf := list.New()
	bridges := make([]dbTypeBridge, colsPerRow)
	scanDest := make([]interface{}, colsPerRow)

	for i, columnType := range columnTypes {
		typ := columnType.DatabaseTypeName()
		factory, hasFactory := dbTypeBridgeFactories[typ]
		if hasFactory {
			bridges[i] = factory()
		} else {
			bridges[i] = &dbBrokenBridge{typ: typ}
		}

		scanDest[i] = bridges[i]
	}

	for {
		if rows.Next() {
			if errScan := rows.Scan(scanDest...); errScan != nil {
				return [][]interface{}{}, errScan
			}

			row := make([]interface{}, colsPerRow)

			for i, bridge := range bridges {
				row[i] = bridge.Result()
			}

			buf.PushBack(row)
		} else if errNx := rows.Err(); errNx == nil {
			break
		} else {
			return nil, errNx
		}
	}

	res := make([][]interface{}, buf.Len())

	for current, i := buf.Front(), 0; current != nil; current = current.Next() {
		res[i] = current.Value.([]interface{})
		i++
	}

	return res, nil
}

// Wrapper around tx.SqlExec() for auto-logging
func (dbw *DBWrapper) SqlExec(tx *sql.Tx, opDescription string, sql string, args ...interface{}) (sql.Result, error) {
	benchmarc := benchmark.NewBenchmark()
	res, err := tx.Exec(sql, args...)
	benchmarc.Stop()

	//DbIoSeconds.WithLabelValues("mysql", opDescription).Observe(benchmarc.Seconds())

	log.WithFields(log.Fields{
		"context":       "sql",
		"benchmark":     benchmarc,
		"affected_rows": prettyPrintedRowsAffected{res},
		"args":          prettyPrintedArgs{args},
		"query":         prettyPrintedSql{sql},
	}).Debug("Finished Exec")

	return res, err
}

// No logging, no benchmarking
func (dbw *DBWrapper) SqlExecQuiet(tx *sql.Tx, opDescription string, sql string, args ...interface{}) (sql.Result, error) {
	res, err := tx.Exec(sql, args...)
	return res, err
}

func formatLogQuery(query string) string {
	r := strings.NewReplacer("\n", " ", "\t", "")
	return strings.TrimSpace(r.Replace(query))
}

// Go bool -> DB bool
var yesNo = map[bool]string{
	true:  "y",
	false: "n",
}

func ConvertValueForDb(in interface{}) interface{} {
	switch value := in.(type) {
	case []byte:
	case string:
	case float64:
	case int64:
	case nil:
		break
	case float32:
		return float64(value)
	case uint:
		return int64(value)
	case uint8:
		return int64(value)
	case uint16:
		return int64(value)
	case uint32:
		return int64(value)
	case uint64:
		return int64(value)
	case int:
		return int64(value)
	case int8:
		return int64(value)
	case int16:
		return int64(value)
	case int32:
		return int64(value)
	case bool:
		return yesNo[value]
	default:
		panic(fmt.Sprintf(
			"bad type %s, expected one of []byte, string, float{32,64}, {,u}int{,8,16,32,64}, bool, nil",
			reflect.TypeOf(in).Name(),
		))
	}

	return in
}

func MakePlaceholderList(x int) string {
	runes := make([]rune, 1+x*2)

	i := 1
	for j := 0; j < x; j++ {
		runes[i] = '?'
		i++

		runes[i] = ','
		i++
	}

	runes[0] = '('
	runes[len(runes)-1] = ')'

	return string(runes)
}
