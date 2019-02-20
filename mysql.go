package icingadb_connection

import (
	"container/list"
	"context"
	"database/sql"
	"git.icinga.com/icingadb/icingadb/benchmark"
	log "github.com/sirupsen/logrus"
	"sync"
	"sync/atomic"
	"time"
)

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
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		res, err := dbw.Db.Query(query, args...)

		if err != nil {
			if !dbw.checkConnection(false) {
				continue
			}
		}

		return res, err
	}
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

// Executes the given function inside a transaction
func (dbw *DBWrapper) sqlTryTransaction(f func(*sql.Tx) error, concurrencySafety bool, quiet bool) error {
	tx, errBegin := dbw.SqlBegin(concurrencySafety, quiet)
	if errBegin != nil {
		return errBegin
	}

	errTx := f(tx)
	if errTx != nil {
		dbw.SqlRollback(tx, quiet)
		return errTx
	}

	return dbw.SqlCommit(tx, quiet)
}

// Wrapper around Db.BeginTx() for auto-logging
func (dbw *DBWrapper) SqlBegin(concurrencySafety bool, quiet bool) (*sql.Tx, error) {
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
func (dbw *DBWrapper) SqlCommit(tx *sql.Tx, quiet bool) error {
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
func (dbw *DBWrapper) SqlRollback(tx *sql.Tx, quiet bool) error {
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
