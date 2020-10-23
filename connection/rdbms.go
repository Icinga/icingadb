// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package connection

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	icingaSql "github.com/Icinga/go-libs/sql"
	"github.com/Icinga/icingadb/config"
	"github.com/Icinga/icingadb/utils"
	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"io/ioutil"
	oldlog "log"
	"net"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var dbObservers = struct {
	begin       prometheus.Observer
	commit      prometheus.Observer
	rollback    prometheus.Observer
	transaction prometheus.Observer
	bulkInsert  prometheus.Observer
	bulkDelete  prometheus.Observer
	bulkUpdate  prometheus.Observer
}{
	DbIoSeconds.WithLabelValues("rdbms", "begin"),
	DbIoSeconds.WithLabelValues("rdbms", "commit"),
	DbIoSeconds.WithLabelValues("rdbms", "rollback"),
	DbIoSeconds.WithLabelValues("rdbms", "transaction"),
	DbIoSeconds.WithLabelValues("rdbms", "Bulk insert"),
	DbIoSeconds.WithLabelValues("rdbms", "Bulk delete"),
	DbIoSeconds.WithLabelValues("rdbms", "Bulk update"),
}

var connectionErrors = []string{
	"server has gone away",
	"no connection to the server",
	"Lost connection",
	"Error while sending",
	"is dead or not enabled",
	"decryption failed or bad record mac",
	"server closed the connection unexpectedly",
	"SSL connection has been closed unexpectedly",
	"Error writing data to the connection",
	"Resource deadlock avoided",
	"Transaction() on null",
	"child connection forced to terminate due to client_idle_limit",
	"query_wait_timeout",
	"reset by peer",
	"Physical connection is not usable",
	"TCP Provider: Error code 0x68",
	"ORA-03114",
	"Packets out of order. Expected",
	"Adaptive Server connection failed",
	"Communication link failure",
	"Deadlock found when trying to get lock",
	"operation timed out",
}

// DbClientOrTransaction is used in SqlFetchAll and SqlFetchAllQuiet.
type DbClientOrTransaction interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type DbClient interface {
	DbClientOrTransaction
	Ping() error
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	Driver() driver.Driver
}

type DbTransaction interface {
	DbClientOrTransaction
	Commit() error
	Rollback() error
	Prepare(query string) (*sql.Stmt, error)
}

func NewDBWrapper(driver string, info *config.DbInfo) (*DBWrapper, error) {
	var dsn *url.URL
	if driver == "postgres" {
		dsn = &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(info.User, info.Password),
			Host:   net.JoinHostPort(info.Host, info.Port),
			Path:   "/" + info.Database,
			RawQuery: "sslmode=disable&" + // https://github.com/lib/pq/issues/1006
				"binary_parameters=yes", // https://github.com/lib/pq/issues/678
		}
	} else {
		dsn = &url.URL{
			User: url.UserPassword(info.User, info.Password),
			Host: "tcp(" + net.JoinHostPort(info.Host, info.Port) + ")",
			Path: "/" + info.Database,
			RawQuery: "innodb_strict_mode=1&sql_mode='STRICT_ALL_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE," +
				"NO_ENGINE_SUBSTITUTION,PIPES_AS_CONCAT,ANSI_QUOTES,ERROR_FOR_DIVISION_BY_ZERO'",
		}
	}

	log.Info("Connecting to database")

	db, err := sql.Open(driver, strings.TrimPrefix(dsn.String(), "//"))
	if err != nil {
		return nil, err
	}

	mysql.SetLogger(oldlog.New(ioutil.Discard, "", 0))

	db.SetMaxOpenConns(info.MaxOpenConns)
	db.SetMaxIdleConns(info.MaxOpenConns)

	dbw := DBWrapper{Db: db, ConnectedAtomic: new(uint32), ConnectionLostCounterAtomic: new(uint32)}
	dbw.ConnectionUpCondition = sync.NewCond(&sync.Mutex{})

	err = dbw.Db.Ping()
	if err != nil {
		log.WithFields(log.Fields{
			"context": "sql",
			"error":   err,
		}).Error("Could not connect to SQL. Trying again")
	}

	go func() {
		for {
			dbw.checkConnection(true)
			time.Sleep(dbw.getConnectionCheckInterval())
		}
	}()

	return &dbw, nil
}

// DBWrapper is a database wrapper including helper functions.
type DBWrapper struct {
	Db                          DbClient
	ConnectedAtomic             *uint32 //uint32 to be able to use atomic operations
	ConnectionUpCondition       *sync.Cond
	ConnectionLostCounterAtomic *uint32 //uint32 to be able to use atomic operations
}

func (dbw *DBWrapper) IsConnected() bool {
	return atomic.LoadUint32(dbw.ConnectedAtomic) != 0
}

func (dbw *DBWrapper) CompareAndSetConnected(connected bool) (swapped bool) {
	if connected {
		return atomic.CompareAndSwapUint32(dbw.ConnectedAtomic, 0, 1)
	} else {
		return atomic.CompareAndSwapUint32(dbw.ConnectedAtomic, 1, 0)
	}
}

func (dbw *DBWrapper) getConnectionCheckInterval() time.Duration {
	if !dbw.IsConnected() {
		v := atomic.LoadUint32(dbw.ConnectionLostCounterAtomic)
		if v < 4 {
			return 5 * time.Second
		} else if v < 8 {
			return 10 * time.Second
		} else if v < 11 {
			return 30 * time.Second
		} else if v < 14 {
			return 60 * time.Second
		} else {
			log.Fatal("Could not connect to SQL for over 5 minutes. Shutting down...")
		}
	}

	return 15 * time.Second
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
			atomic.AddUint32(dbw.ConnectionLostCounterAtomic, 1)

			log.WithFields(log.Fields{
				"context": "sql",
				"error":   err,
			}).Debugf("SQL connection lost. Trying again in %s", dbw.getConnectionCheckInterval())
		}

		return false
	} else {
		if dbw.CompareAndSetConnected(true) {
			log.Info("SQL connection established")
			atomic.StoreUint32(dbw.ConnectionLostCounterAtomic, 0)
			dbw.ConnectionUpCondition.Broadcast()
		}

		return true
	}
}

func (dbw *DBWrapper) isConnectionError(err error) bool {
	errString := err.Error()
	for _, str := range connectionErrors {
		if strings.Contains(errString, str) {
			log.WithFields(log.Fields{
				"context": "sql",
				"error":   errString,
			}).Debug("Got connection error. Trying again")
			return true
		}
	}

	return !dbw.checkConnection(false)
}

func (dbw *DBWrapper) WaitForConnection() {
	dbw.ConnectionUpCondition.L.Lock()
	dbw.ConnectionUpCondition.Wait()
	dbw.ConnectionUpCondition.L.Unlock()
}

func (dbw *DBWrapper) WithRetry(f func() (sql.Result, error)) (sql.Result, error) {
	for {
		res, err := f()

		if err != nil {
			if isSerializationFailure(err) {
				continue
			} else {
				return nil, err
			}
		}

		return res, err
	}
}

func (dbw *DBWrapper) SqlQuery(query string, args ...interface{}) (*sql.Rows, error) {
	DbOperationsQuery.Inc()
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		res, err := dbw.Db.Query(query, args...)

		if err != nil {
			if dbw.isConnectionError(err) {
				continue
			}
		}

		return res, err
	}
}

// SqlBegin is a wrapper around Db.BeginTx() for auto-logging.
func (dbw *DBWrapper) SqlBegin(concurrencySafety bool, quiet bool) (DbTransaction, error) {
	DbOperationsBegin.Inc()
	var isoLvl sql.IsolationLevel
	if concurrencySafety {
		isoLvl = sql.LevelSerializable
	} else {
		isoLvl = sql.LevelReadCommitted
	}

	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		var err error
		var tx DbTransaction
		if quiet {
			tx, err = dbw.Db.BeginTx(context.Background(), &sql.TxOptions{Isolation: isoLvl})
		} else {
			benchmarc := utils.NewBenchmark()
			tx, err = dbw.Db.BeginTx(context.Background(), &sql.TxOptions{Isolation: isoLvl})
			benchmarc.Stop()

			dbObservers.begin.Observe(benchmarc.Seconds())

			log.WithFields(log.Fields{
				"context":   "sql",
				"benchmark": benchmarc,
			}).Debug("BEGIN transaction")
		}

		if err != nil {
			if dbw.isConnectionError(err) {
				continue
			}
		}

		return tx, err
	}
}

// SqlCommit is a wrapper around tx.Commit() for auto-logging.
func (dbw *DBWrapper) SqlCommit(tx DbTransaction, quiet bool) error {
	DbOperationsCommit.Inc()
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		var err error
		if quiet {
			err = tx.Commit()
		} else {
			benchmarc := utils.NewBenchmark()
			err = tx.Commit()
			benchmarc.Stop()

			dbObservers.commit.Observe(benchmarc.Seconds())

			log.WithFields(log.Fields{
				"context":   "sql",
				"benchmark": benchmarc,
			}).Debug("COMMIT transaction")
		}

		if err != nil {
			if dbw.isConnectionError(err) {
				continue
			}
		}

		return err
	}
}

// SqlRollback is a wrapper around tx.Rollback() for auto-logging.
func (dbw *DBWrapper) SqlRollback(tx DbTransaction, quiet bool) error {
	DbOperationsRollback.Inc()
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		var err error
		if !quiet {
			benchmarc := utils.NewBenchmark()
			err = tx.Rollback()
			benchmarc.Stop()

			dbObservers.rollback.Observe(benchmarc.Seconds())

			log.WithFields(log.Fields{
				"context":   "sql",
				"benchmark": benchmarc,
			}).Debug("ROLLBACK transaction")
		} else {
			err = tx.Rollback()
		}

		if err != nil {
			if dbw.isConnectionError(err) {
				continue
			}
		}

		return err
	}
}

// SqlExec is a wrapper around sql.Exec() for auto-logging.
func (dbw *DBWrapper) SqlExec(opObserver prometheus.Observer, sql string, args ...interface{}) (sql.Result, error) {
	return dbw.sqlExecInternal(dbw.Db, opObserver, sql, false, args...)
}

// SqlExecQuiet is like SqlExec, but doesn't log or benchmark.
func (dbw *DBWrapper) SqlExecQuiet(opObserver prometheus.Observer, sql string, args ...interface{}) (sql.Result, error) {
	return dbw.sqlExecInternal(dbw.Db, opObserver, sql, true, args...)
}

// SqlExecTx is a wrapper around tx.Exec() for auto-logging.
func (dbw *DBWrapper) SqlExecTx(tx DbTransaction, opObserver prometheus.Observer, sql string, args ...interface{}) (sql.Result, error) {
	return dbw.sqlExecInternal(tx, opObserver, sql, false, args...)
}

// SqlExecTxQuiet is like SqlExecTx, but doesn't log or benchmark.
func (dbw *DBWrapper) SqlExecTxQuiet(tx DbTransaction, opObserver prometheus.Observer, sql string, args ...interface{}) (sql.Result, error) {
	return dbw.sqlExecInternal(tx, opObserver, sql, true, args...)
}

func (dbw *DBWrapper) SqlFetchAll(queryObserver prometheus.Observer, rowType interface{}, query string, args ...interface{}) (interface{}, error) {
	return dbw.sqlFetchAllInternal(dbw.Db, queryObserver, query, rowType, false, args...)
}

// SqlExecStmt is a wrapper around stmt.Exec() for auto-logging. sql is just for logging.
func (dbw *DBWrapper) SqlExecStmt(stmt *sql.Stmt, opObserver prometheus.Observer, sql string, args ...interface{}) (sql.Result, error) {
	return dbw.sqlExecInternal(sqlStmtRunner{stmt}, opObserver, sql, false, args...)
}

type sqlStmtRunner struct {
	stmt *sql.Stmt
}

var _ DbTransaction = sqlStmtRunner{}

func (ssr sqlStmtRunner) Query(string, ...interface{}) (*sql.Rows, error) {
	panic("don't call me")
}

func (ssr sqlStmtRunner) Exec(_ string, args ...interface{}) (sql.Result, error) {
	return ssr.stmt.Exec(args...)
}

func (ssr sqlStmtRunner) Commit() error {
	panic("don't call me")
}

func (ssr sqlStmtRunner) Rollback() error {
	panic("don't call me")
}

func (ssr sqlStmtRunner) Prepare(string) (*sql.Stmt, error) {
	panic("don't call me")
}

// sqlExecInternal is a wrapper around sql.Exec() for auto-logging.
func (dbw *DBWrapper) sqlExecInternal(db DbClientOrTransaction, opObserver prometheus.Observer, sql string, quiet bool, args ...interface{}) (sql.Result, error) {
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		var benchmarc *utils.Benchmark
		if !quiet {
			benchmarc = utils.NewBenchmark()
		}

		res, err := db.Exec(sql, args...)
		DbOperationsExec.Inc()

		if !quiet {
			benchmarc.Stop()
		}

		if !quiet {
			opObserver.Observe(benchmarc.Seconds())
			log.WithFields(log.Fields{
				"context":       "sql",
				"benchmark":     benchmarc,
				"affected_rows": prettyPrintedRowsAffected{res},
				"args":          prettyPrintedArgs{args},
				"query":         prettyPrintedSql{sql},
			}).Debug("Finished Exec")
		}

		if err != nil {
			if dbw.isConnectionError(err) {
				continue
			}
		}

		return res, err
	}
}

// sqlFetchAllInternal is a wrapper around Db.SqlQuery() for auto-logging.
func (dbw *DBWrapper) sqlFetchAllInternal(db DbClientOrTransaction, queryObserver prometheus.Observer, query string, rowType interface{}, quiet bool, args ...interface{}) (interface{}, error) {
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		res, err := sqlTryFetchAll(db, queryObserver, query, rowType, quiet, args...)

		if err != nil {
			if _, isDb := db.(*sql.DB); isDb {
				if dbw.isConnectionError(err) {
					continue
				}
			}
		}

		return res, err
	}
}

func sqlTryFetchAll(db DbClientOrTransaction, queryObserver prometheus.Observer, query string, rowType interface{}, quiet bool, args ...interface{}) (interface{}, error) {
	var benchmarc *utils.Benchmark
	if !quiet {
		benchmarc = utils.NewBenchmark()
	}
	rows, errQuery := db.Query(query, args...)
	if !quiet {
		benchmarc.Stop()
	}

	rowsCount := 0

	defer func() {
		if !quiet {
			queryObserver.Observe(benchmarc.Seconds())
			log.WithFields(log.Fields{
				"context":       "sql",
				"benchmark":     benchmarc,
				"query":         prettyPrintedSql{query},
				"args":          prettyPrintedArgs{args},
				"affected_Rows": rowsCount,
			}).Debug("Finished FetchAll")
		}
	}()

	if errQuery != nil {
		return [][]interface{}{}, errQuery
	}

	defer rows.Close()

	res, errFR := icingaSql.FetchRowsAsStructSlice(rows, rowType, -1)
	if errFR == nil {
		rowsCount = reflect.ValueOf(res).Len()
	} else {
		rowsCount = 0
	}

	return res, errFR
}

// sqlTransaction executes the given function inside a transaction.
func (dbw DBWrapper) SqlTransaction(concurrencySafety bool, retryOnConnectionFailure bool, quiet bool, f func(DbTransaction) error) error {
	DbTransactions.Inc()
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		var benchmarc *utils.Benchmark
		if !quiet {
			benchmarc = utils.NewBenchmark()
		}
		errTx := dbw.sqlTryTransaction(f, concurrencySafety, false)
		if !quiet {
			benchmarc.Stop()
			dbObservers.transaction.Observe(benchmarc.Seconds())

			log.WithFields(log.Fields{
				"context":   "sql",
				"benchmark": benchmarc,
			}).Debug("Executed transaction")
		}

		if errTx != nil {
			//TODO: Do this only for concurrencySafety = true, once we figure out the serialization errors.
			if isSerializationFailure(errTx) {
				if !quiet {
					log.WithFields(log.Fields{
						"context": "sql",
						"error":   errTx,
					}).Debug("Repeating transaction")
				}
				continue
			}

			if dbw.isConnectionError(errTx) {
				if retryOnConnectionFailure {
					continue
				} else {
					return DbConnectionError{"Transaction failed duo to a connection error"}
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

// sqlTryTransaction executes the given function inside a transaction.
func (dbw *DBWrapper) sqlTryTransaction(f func(transaction DbTransaction) error, concurrencySafety bool, quiet bool) error {
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

func (dbw *DBWrapper) SqlFetchIds(envId []byte, table string, field string) ([]string, error) {
	DbFetchIds.Inc()
	var keys []string
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		rows, err := dbw.SqlQuery(
			fmt.Sprintf(
				"SELECT %s FROM %s WHERE environment_id=%s AND NOT %s=%s",
				field, EscapeName(dbw.Db, table), Placeholders(dbw.Db, 0, 1), field, Placeholders(dbw.Db, 1, 1),
			),
			envId, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		)

		if err != nil {
			if dbw.isConnectionError(err) {
				continue
			}

			return nil, err
		}

		defer rows.Close()

		for rows.Next() {
			var id []byte

			err = rows.Scan(&id)
			if err != nil {
				return nil, err
			}

			keys = append(keys, utils.DecodeChecksum(id))
		}

		err = rows.Err()
		if err != nil {
			return nil, err
		}

		return keys, nil
	}
}

func (dbw *DBWrapper) SqlFetchChecksums(table string, ids []string) (map[string]map[string]string, error) {
	DbFetchChecksums.Inc()
	var checksums = map[string]map[string]string{}
	done := make(chan struct{})
	//TODO: Don't do this hardcoded - Chunksize
	for bulk := range utils.ChunkKeys(done, ids, 1000) {
		//TODO: This should be done in parallel

		ids := make([]interface{}, 0, len(bulk))
		for _, id := range bulk {
			ids = append(ids, utils.EncodeChecksum(id))
		}

		rows, err := dbw.SqlQuery(
			fmt.Sprintf(
				"SELECT id, properties_checksum FROM %s WHERE id IN (%s)",
				EscapeName(dbw.Db, table), Placeholders(dbw.Db, 0, len(bulk)),
			),
			ids...,
		)
		if err != nil {
			if dbw.isConnectionError(err) {
				continue
			}

			return nil, err
		}

		defer rows.Close()

		for rows.Next() {
			var id []byte
			var propertiesChecksum []byte

			err = rows.Scan(&id, &propertiesChecksum)
			if err != nil {
				return nil, err
			}

			checksums[utils.DecodeChecksum(id)] = map[string]string{
				"properties_checksum": utils.DecodeChecksum(propertiesChecksum),
			}
		}

		err = rows.Err()
		if err != nil {
			return nil, err
		}

		rows.Close()
	}

	return checksums, nil
}

func (dbw *DBWrapper) SqlBulkInsert(rows []Row, stmt *BulkInsertStmt) error {
	if len(rows) == 0 {
		return nil
	}

	finalRows := make([]Row, 0)
	for _, r := range rows {
		fr, _ := r.GetFinalRows()
		finalRows = append(finalRows, fr...)
	}

	rows = finalRows

	DbBulkInserts.Inc()

	group := errgroup.Group{}

	for _, c := range ChunkRows(rows, 500) {
		chunk := c
		group.Go(func() error {
			return dbw.SqlTransaction(false, true, false, func(tx DbTransaction) error {
				query := Replace(dbw.Db, stmt.Table, stmt.Fields...)
				smt, errPp := tx.Prepare(query)

				if errPp != nil {
					return errPp
				}

				defer smt.Close()

				for _, row := range chunk {
					if _, errEx := dbw.SqlExecStmt(smt, dbObservers.bulkInsert, query, row.InsertValues()...); errEx != nil {
						return errEx
					}
				}

				return nil
			})
		})
	}

	return group.Wait()
}

func (dbw *DBWrapper) SqlBulkDelete(keys []string, stmt *BulkDeleteStmt) error {
	if len(keys) == 0 {
		return nil
	}

	DbBulkDeletes.Inc()

	done := make(chan struct{})
	defer close(done)

	//TODO: Don't do this hardcoded - Chunksize
	for bulk := range utils.ChunkKeys(done, keys, 1000) {
		values := make([]interface{}, len(bulk))

		for i, key := range bulk {
			values[i] = utils.EncodeChecksum(key)
		}

		query := fmt.Sprintf(
			"DELETE FROM %s WHERE %s IN (%s)",
			EscapeName(dbw.Db, stmt.Table), EscapeName(dbw.Db, stmt.PrimaryKey), Placeholders(dbw.Db, 0, len(bulk)),
		)

		_, err := dbw.WithRetry(func() (result sql.Result, e error) {
			return dbw.SqlExec(dbObservers.bulkDelete, query, values...)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (dbw *DBWrapper) SqlBulkUpdate(rows []Row, stmt *BulkUpdateStmt) error {
	if len(rows) == 0 {
		return nil
	}

	DbBulkUpdates.Inc()

	group := errgroup.Group{}

	for _, c := range ChunkRows(rows, 500) {
		chunk := c
		group.Go(func() error {
			return dbw.SqlTransaction(false, true, false, func(tx DbTransaction) error {
				query := Replace(dbw.Db, stmt.Table, stmt.Fields...)
				smt, errPp := tx.Prepare(query)

				if errPp != nil {
					return errPp
				}

				defer smt.Close()

				for _, row := range chunk {
					_, errEx := dbw.SqlExecStmt(smt, dbObservers.bulkUpdate, query, row.InsertValues()...)
					if errEx != nil {
						return errEx
					}
				}

				return nil
			})
		})
	}

	return group.Wait()
}
