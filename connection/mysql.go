package connection

import (
	"container/list"
	"context"
	"database/sql"
	"fmt"
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-utils"
	log "github.com/sirupsen/logrus"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// This is used in SqlFetchAll and SqlFetchAllQuiet
type DbClientOrTransaction interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type DbClient interface {
	DbClientOrTransaction
	Ping() error
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

type DbTransaction interface {
	DbClientOrTransaction
	Commit() error
	Rollback() error
}

func NewDBWrapper(dbDsn string) (*DBWrapper, error) {
	db, err := mkMysql("mysql", dbDsn)

	if err != nil {
		return nil, err
	}

	dbw := DBWrapper{Db: db, ConnectedAtomic: new(uint32), ConnectionLostCounterAtomic: new(uint32)}
	dbw.ConnectionUpCondition = sync.NewCond(&sync.Mutex{})

	err = dbw.Db.Ping()
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			dbw.checkConnection(true)
			time.Sleep(dbw.getConnectionCheckInterval())
		}
	}()

	return &dbw, nil
}

// Database wrapper including helper functions
type DBWrapper struct {
	Db                   		 DbClient
	ConnectedAtomic  		     *uint32 //uint32 to be able to use atomic operations
	ConnectionUpCondition		 *sync.Cond
	ConnectionLostCounterAtomic	 *uint32 //uint32 to be able to use atomic operations
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

func (dbw *DBWrapper) WaitForConnection() {
	dbw.ConnectionUpCondition.L.Lock()
	dbw.ConnectionUpCondition.Wait()
	dbw.ConnectionUpCondition.L.Unlock()
}

func (dbw *DBWrapper) WithRetry(f func() (sql.Result, error)) (sql.Result, error) {
	for {
		res, err := f()

		if err != nil {
			if isRetryableError(err) {
				continue
			} else {
				return nil, err
			}
		}

		return res, err
	}
}

func (dbw *DBWrapper) SqlQuery(query string, args ...interface{}) (*sql.Rows, error) {
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		res, err := dbw.Db.Query(query, args...)
		DbOperationsQuery.Inc()

		if err != nil {
			if !dbw.checkConnection(false) {
				continue
			}
		}

		return res, err
	}
}

// Wrapper around Db.BeginTx() for auto-logging
func (dbw *DBWrapper) SqlBegin(concurrencySafety bool, quiet bool) (DbTransaction, error) {
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
			benchmarc := icingadb_utils.NewBenchmark()
			tx, err = dbw.Db.BeginTx(context.Background(), &sql.TxOptions{Isolation: isoLvl})
			benchmarc.Stop()

			DbIoSeconds.WithLabelValues("mysql", "begin").Observe(benchmarc.Seconds())

			log.WithFields(log.Fields{
				"context":   "sql",
				"benchmark": benchmarc,
			}).Debug("BEGIN transaction")
		}

		if err != nil {
			if !dbw.checkConnection(false) {
				continue
			}
		}

		return tx, err
	}
}

// Wrapper around tx.Commit() for auto-logging
func (dbw *DBWrapper) SqlCommit(tx DbTransaction, quiet bool) error {
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		var err error
		if quiet {
			err = tx.Commit()
		} else {
			benchmarc := icingadb_utils.NewBenchmark()
			err = tx.Commit()
			benchmarc.Stop()

			DbIoSeconds.WithLabelValues("mysql", "commit").Observe(benchmarc.Seconds())

			log.WithFields(log.Fields{
				"context":   "sql",
				"benchmark": benchmarc,
			}).Debug("COMMIT transaction")
		}

		if err != nil {
			if !dbw.checkConnection(false) {
				continue
			}
		}

		return err
	}
}

// Wrapper around tx.Rollback() for auto-logging
func (dbw *DBWrapper) SqlRollback(tx DbTransaction, quiet bool) error {
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		var err error
		if !quiet {
			benchmarc := icingadb_utils.NewBenchmark()
			err = tx.Rollback()
			benchmarc.Stop()

			DbIoSeconds.WithLabelValues("mysql", "rollback").Observe(benchmarc.Seconds())

			log.WithFields(log.Fields{
				"context":   "sql",
				"benchmark": benchmarc,
			}).Debug("ROLLBACK transaction")
		} else {
			err = tx.Rollback()
		}

		if err != nil {
			if !dbw.checkConnection(false) {
				continue
			}
		}

		return err
	}
}

// Wrapper around sql.Exec() for auto-logging
func (dbw *DBWrapper) SqlExec(opDescription string, sql string, args ...interface{}) (sql.Result, error) {
	return dbw.sqlExecInternal(dbw.Db, opDescription, sql, false, args...)
}

// No logging, no benchmarking
func (dbw *DBWrapper) SqlExecQuiet(opDescription string, sql string, args ...interface{}) (sql.Result, error) {
	return dbw.sqlExecInternal(dbw.Db, opDescription, sql, true, args...)
}

// Wrapper around tx.Exec() for auto-logging
func (dbw *DBWrapper) SqlExecTx(tx DbTransaction, opDescription string, sql string, args ...interface{}) (sql.Result, error) {
	return dbw.sqlExecInternal(tx, opDescription, sql, false, args...)
}

// No logging, no benchmarking
func (dbw *DBWrapper) SqlExecTxQuiet(tx DbTransaction, opDescription string, sql string, args ...interface{}) (sql.Result, error) {
	return dbw.sqlExecInternal(tx, opDescription, sql, true, args...)
}

func (dbw *DBWrapper) SqlFetchAll(queryDescription string, query string, args ...interface{}) ([][]interface{}, error) {
	return dbw.sqlFetchAllInternal(dbw.Db, queryDescription, query, false, args...)
}

func (dbw *DBWrapper) SqlFetchAllQuiet(queryDescription string, query string, args ...interface{}) ([][]interface{}, error) {
	return dbw.sqlFetchAllInternal(dbw.Db, queryDescription, query, true, args...)
}

func (dbw *DBWrapper) SqlFetchAllTx(tx DbTransaction, queryDescription string, query string, args ...interface{}) ([][]interface{}, error) {
	return dbw.sqlFetchAllInternal(tx, queryDescription, query, false, args...)
}

func (dbw *DBWrapper) SqlFetchAllTxQuiet(tx DbTransaction, queryDescription string, query string, args ...interface{}) ([][]interface{}, error) {
	return dbw.sqlFetchAllInternal(tx, queryDescription, query, true, args...)
}

// Wrapper around sql.Exec() for auto-logging
func (dbw *DBWrapper) sqlExecInternal(db DbClientOrTransaction, opDescription string, sql string, quiet bool, args ...interface{}) (sql.Result, error) {
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		var benchmarc *icingadb_utils.Benchmark
		if !quiet {
			benchmarc = icingadb_utils.NewBenchmark()
		}
		res, err := db.Exec(sql, args...)
		DbOperationsExec.Inc()
		if !quiet {
			benchmarc.Stop()
		}

		if !quiet {
			DbIoSeconds.WithLabelValues("mysql", opDescription).Observe(benchmarc.Seconds())
			log.WithFields(log.Fields{
				"context":       "sql",
				"benchmark":     benchmarc,
				"affected_rows": prettyPrintedRowsAffected{res},
				"args":          prettyPrintedArgs{args},
				"query":         prettyPrintedSql{sql},
			}).Debug("Finished Exec")
		}


		if err != nil {
			if !dbw.checkConnection(false) {
				continue
			}
		}

		return res, err
	}
}

// Wrapper around Db.SqlQuery() for auto-logging
func (dbw *DBWrapper) sqlFetchAllInternal(db DbClientOrTransaction, queryDescription string, query string, quiet bool, args ...interface{}) ([][]interface{}, error) {
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		res, err := sqlTryFetchAll(db, queryDescription, query, quiet, args...)

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

func sqlTryFetchAll(db DbClientOrTransaction, queryDescription string, query string, quiet bool, args ...interface{}) ([][]interface{}, error) {
	var benchmarc *icingadb_utils.Benchmark
	if !quiet {
		benchmarc = icingadb_utils.NewBenchmark()
	}
	rows, errQuery := db.Query(query, args...)
	DbOperationsQuery.Inc()
	if !quiet {
		benchmarc.Stop()
	}

	rowsCount := 0

	defer func() {
		if !quiet {
			DbIoSeconds.WithLabelValues("mysql", queryDescription).Observe(benchmarc.Seconds())
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

// sqlTransaction executes the given function inside a transaction.
func (dbw DBWrapper) SqlTransaction(concurrencySafety bool, retryOnConnectionFailure bool, quiet bool, f func(DbTransaction) error) error {
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		var benchmarc *icingadb_utils.Benchmark
		if !quiet {
			benchmarc = icingadb_utils.NewBenchmark()
		}
		errTx := dbw.sqlTryTransaction(f, concurrencySafety, false)
		if !quiet {
			benchmarc.Stop()
			DbIoSeconds.WithLabelValues("mysql", "transaction").Observe(benchmarc.Seconds())

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

func (dbw *DBWrapper) SqlFetchIds(envId []byte, table string) ([]string, error) {
	var keys []string
	for {
		if !dbw.IsConnected() {
			dbw.WaitForConnection()
			continue
		}

		rows, err := dbw.SqlQuery(fmt.Sprintf("SELECT id FROM %s WHERE env_id=(X'%s')", table, icingadb_utils.DecodeChecksum(envId)))

		if err != nil {
			if !dbw.checkConnection(false) {
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

			keys = append(keys, icingadb_utils.DecodeChecksum(id))
		}

		err = rows.Err()
		if err != nil {
			return nil, err
		}

		return keys, nil
	}
}

func (dbw *DBWrapper) SqlFetchChecksums(table string, ids []string) (map[string]map[string]string, error) {
	var checksums = map[string]map[string]string{}

	done := make(chan struct{})
	//TODO: Don't do this hardcoded - Chunksize
	for bulk := range icingadb_utils.ChunkKeys(done, ids, 1000) {
		//TODO: This should be done in parallel
		query := fmt.Sprintf("SELECT id, properties_checksum FROM %s WHERE id IN (X'%s')", table, strings.Join(bulk, "', X'"))
		rows, err := dbw.SqlQuery(query)

		if err != nil {
			if !dbw.checkConnection(false) {
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

			checksums[icingadb_utils.DecodeChecksum(id)] = map[string]string{
				"properties_checksum": icingadb_utils.DecodeChecksum(propertiesChecksum),
			}
		}

		err = rows.Err()
		if err != nil {
			return nil, err
		}
	}

	return checksums, nil
}

func (dbw *DBWrapper) SqlBulkInsert(rows []configobject.Row, stmt *BulkInsertStmt) error {
	if len(rows) == 0 {
		return nil
	}

	placeholders := make([]string, len(rows))
	values := make([]interface{}, len(rows)*stmt.NumField)
	j := 0

	for i, r := range rows {
		placeholders[i] = stmt.Placeholder

		for _, v := range r.InsertValues() {
			values[j] = v
			j++
		}
	}

	query := fmt.Sprintf(stmt.Format, strings.Join(placeholders, ", "))

	_, err := dbw.WithRetry(func() (result sql.Result, e error) {
		return dbw.SqlExec("Bulk insert", query, values...)
	})

	if err != nil {
		return err
	}

	return nil
}

func (dbw *DBWrapper) SqlBulkDelete(keys []string, stmt *BulkDeleteStmt) error {
	if len(keys) == 0 {
		return nil
	}

	done := make(chan struct{})
	defer close(done)

	//TODO: Don't do this hardcoded - Chunksize
	for bulk := range icingadb_utils.ChunkKeys(done, keys, 1000) {
		placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(bulk)), ", ")
		values := make([]interface{}, len(bulk))

		for i, key := range bulk {
			values[i] = icingadb_utils.Checksum(key)
		}
		query := fmt.Sprintf(stmt.Format, placeholders)

		_, err := dbw.WithRetry(func() (result sql.Result, e error) {
			return dbw.SqlExec("Bulk delete", query, values...)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (dbw *DBWrapper) SqlBulkUpdate(rows []configobject.Row, stmt *BulkUpdateStmt) error {
	if len(rows) == 0 {
		return nil
	}

	placeholders := make([]string, len(rows))
	values := make([]interface{}, len(rows)*stmt.NumField)
	j := 0

	for i, r := range rows {
		placeholders[i] = stmt.Placeholder

		for _, v := range r.InsertValues() {
			values[j] = v
			j++
		}
	}

	query := fmt.Sprintf(stmt.Format, strings.Join(placeholders, ", "))

	_, err := dbw.WithRetry(func() (result sql.Result, e error) {
		return dbw.SqlExec("Bulk update", query, values...)
	})
	if err != nil {
		return err
	}

	return nil
}