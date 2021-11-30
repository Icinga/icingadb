package icingadb

import (
	"context"
	"database/sql/driver"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/backoff"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/periodic"
	"github.com/icinga/icingadb/pkg/retry"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"reflect"
	"strings"
	"sync"
	"time"
)

// DB is a wrapper around sqlx.DB with bulk execution,
// statement building, streaming and logging capabilities.
type DB struct {
	*sqlx.DB

	Options *Options

	logger            *logging.Logger
	tableSemaphores   map[string]*semaphore.Weighted
	tableSemaphoresMu sync.Mutex
}

// Options define user configurable database options.
type Options struct {
	// Maximum number of open connections to the database.
	MaxConnections int `yaml:"max_connections" default:"16"`

	// Maximum number of connections per table,
	// regardless of what the connection is actually doing,
	// e.g. INSERT, UPDATE, DELETE.
	MaxConnectionsPerTable int `yaml:"max_connections_per_table" default:"8"`

	// MaxPlaceholdersPerStatement defines the maximum number of placeholders in an
	// INSERT, UPDATE or DELETE statement. Theoretically, MySQL can handle up to 2^16-1 placeholders,
	// but this increases the execution time of queries and thus reduces the number of queries
	// that can be executed in parallel in a given time.
	// The default is 2^13, which in our tests showed the best performance in terms of execution time and parallelism.
	MaxPlaceholdersPerStatement int `yaml:"max_placeholders_per_statement" default:"8192"`

	// MaxRowsPerTransaction defines the maximum number of rows per transaction.
	// The default is 2^13, which in our tests showed the best performance in terms of execution time and parallelism.
	MaxRowsPerTransaction int `yaml:"max_rows_per_transaction" default:"8192"`
}

// Validate checks constraints in the supplied database options and returns an error if they are violated.
func (o *Options) Validate() error {
	if o.MaxConnections == 0 {
		return errors.New("max_connections cannot be 0. Configure a value greater than zero, or use -1 for no connection limit")
	}
	if o.MaxConnectionsPerTable < 1 {
		return errors.New("max_connections_per_table must be at least 1")
	}
	if o.MaxPlaceholdersPerStatement < 1 {
		return errors.New("max_placeholders_per_statement must be at least 1")
	}
	if o.MaxRowsPerTransaction < 1 {
		return errors.New("max_rows_per_transaction must be at least 1")
	}

	return nil
}

// NewDb returns a new icingadb.DB wrapper for a pre-existing *sqlx.DB.
func NewDb(db *sqlx.DB, logger *logging.Logger, options *Options) *DB {
	return &DB{
		DB:              db,
		logger:          logger,
		Options:         options,
		tableSemaphores: make(map[string]*semaphore.Weighted),
	}
}

// BuildColumns returns all columns of the given struct.
func (db *DB) BuildColumns(subject interface{}) []string {
	fields := db.Mapper.TypeMap(reflect.TypeOf(subject)).Names
	columns := make([]string, 0, len(fields))
	for _, f := range fields {
		if f.Field.Tag == "" {
			continue
		}
		columns = append(columns, f.Name)
	}

	return columns
}

// BuildDeleteStmt returns a DELETE statement for the given struct.
func (db *DB) BuildDeleteStmt(from interface{}) string {
	return fmt.Sprintf(
		`DELETE FROM "%s" WHERE id IN (?)`,
		utils.TableName(from),
	)
}

// BuildInsertStmt returns an INSERT INTO statement for the given struct.
func (db *DB) BuildInsertStmt(into interface{}) (string, int) {
	columns := db.BuildColumns(into)

	return fmt.Sprintf(
		`INSERT INTO "%s" (%s) VALUES (%s)`,
		utils.TableName(into),
		strings.Join(columns, ", "),
		fmt.Sprintf(":%s", strings.Join(columns, ", :")),
	), len(columns)
}

// BuildInsertIgnoreStmt returns an INSERT statement for the specified struct for
// which the database ignores rows that have already been inserted.
func (db *DB) BuildInsertIgnoreStmt(into interface{}) (string, int) {
	table := utils.TableName(into)
	columns := db.BuildColumns(into)
	var clause string

	switch db.DriverName() {
	case "icingadb-mysql":
		// MySQL treats UPDATE id = id as a no-op.
		clause = "ON DUPLICATE KEY UPDATE id = id"
	case "icingadb-pgsql":
		clause = fmt.Sprintf("ON CONFLICT ON CONSTRAINT pk_%s DO NOTHING", table)
	}

	return fmt.Sprintf(
		`INSERT INTO "%s" (%s) VALUES (%s) %s`,
		table,
		strings.Join(columns, ", "),
		fmt.Sprintf(":%s", strings.Join(columns, ", :")),
		clause,
	), len(columns)
}

// BuildSelectStmt returns a SELECT query that creates the FROM part from the given table struct
// and the column list from the specified columns struct.
func (db *DB) BuildSelectStmt(table interface{}, columns interface{}) string {
	q := fmt.Sprintf(
		`SELECT %s FROM "%s"`,
		strings.Join(db.BuildColumns(columns), ", "),
		utils.TableName(table),
	)

	if scoper, ok := table.(contracts.Scoper); ok {
		where, _ := db.BuildWhere(scoper.Scope())
		q += ` WHERE ` + where
	}

	return q
}

// BuildUpdateStmt returns an UPDATE statement for the given struct.
func (db *DB) BuildUpdateStmt(update interface{}) (string, int) {
	columns := db.BuildColumns(update)
	set := make([]string, 0, len(columns))

	for _, col := range columns {
		set = append(set, fmt.Sprintf("%s = :%s", col, col))
	}

	return fmt.Sprintf(
		`UPDATE "%s" SET %s WHERE id = :id`,
		utils.TableName(update),
		strings.Join(set, ", "),
	), len(columns) + 1 // +1 because of WHERE id = :id
}

// BuildUpsertStmt returns an upsert statement for the given struct.
func (db *DB) BuildUpsertStmt(subject interface{}) (stmt string, placeholders int) {
	insertColumns := db.BuildColumns(subject)
	table := utils.TableName(subject)
	var updateColumns []string

	if upserter, ok := subject.(contracts.Upserter); ok {
		updateColumns = db.BuildColumns(upserter.Upsert())
	} else {
		updateColumns = insertColumns
	}

	var clause, setFormat string
	switch db.DriverName() {
	case "icingadb-mysql":
		clause = "ON DUPLICATE KEY UPDATE"
		setFormat = "%[1]s = VALUES(%[1]s)"
	case "icingadb-pgsql":
		clause = fmt.Sprintf("ON CONFLICT ON CONSTRAINT pk_%s DO UPDATE SET", table)
		setFormat = "%[1]s = EXCLUDED.%[1]s"
	}

	set := make([]string, 0, len(updateColumns))

	for _, col := range updateColumns {
		set = append(set, fmt.Sprintf(setFormat, col))
	}

	return fmt.Sprintf(
		`INSERT INTO "%s" (%s) VALUES (%s) %s %s`,
		table,
		strings.Join(insertColumns, ","),
		fmt.Sprintf(":%s", strings.Join(insertColumns, ",:")),
		clause,
		strings.Join(set, ","),
	), len(insertColumns)
}

// BuildWhere returns a WHERE clause with named placeholder conditions built from the specified struct
// combined with the AND operator.
func (db *DB) BuildWhere(subject interface{}) (string, int) {
	columns := db.BuildColumns(subject)
	where := make([]string, 0, len(columns))
	for _, col := range columns {
		where = append(where, fmt.Sprintf("%s = :%s", col, col))
	}

	return strings.Join(where, ` AND `), len(columns)
}

// BulkExec bulk executes queries with a single slice placeholder in the form of `IN (?)`.
// Takes in up to the number of arguments specified in count from the arg stream,
// derives and expands a query and executes it with this set of arguments until the arg stream has been processed.
// The derived queries are executed in a separate goroutine with a weighting of 1
// and can be executed concurrently to the extent allowed by the semaphore passed in sem.
// Arguments for which the query ran successfully will be streamed on the succeeded channel.
func (db *DB) BulkExec(ctx context.Context, query string, count int, sem *semaphore.Weighted, arg <-chan interface{}, succeeded chan<- interface{}) error {
	var counter com.Counter
	defer db.log(ctx, query, &counter).Stop()

	g, ctx := errgroup.WithContext(ctx)
	// Use context from group.
	bulk := com.Bulk(ctx, arg, count)

	g.Go(func() error {
		g, ctx := errgroup.WithContext(ctx)

		for b := range bulk {
			if err := sem.Acquire(ctx, 1); err != nil {
				return errors.Wrap(err, "can't acquire semaphore")
			}

			g.Go(func(b []interface{}) func() error {
				return func() error {
					defer sem.Release(1)

					return retry.WithBackoff(
						ctx,
						func(context.Context) error {
							stmt, args, err := sqlx.In(query, b)
							if err != nil {
								return errors.Wrapf(err, "can't build placeholders for %q", query)
							}

							stmt = db.Rebind(stmt)
							_, err = db.ExecContext(ctx, stmt, args...)
							if err != nil {
								return internal.CantPerformQuery(err, query)
							}

							counter.Add(uint64(len(b)))

							if succeeded != nil {
								for _, row := range b {
									select {
									case <-ctx.Done():
										return ctx.Err()
									case succeeded <- row:
									}
								}
							}

							return nil
						},
						IsRetryable,
						backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second),
						retry.Settings{},
					)
				}
			}(b))
		}

		return g.Wait()
	})

	return g.Wait()
}

// NamedBulkExec bulk executes queries with named placeholders in a VALUES clause most likely
// in the format INSERT ... VALUES. Takes in up to the number of entities specified in count
// from the arg stream, derives and executes a new query with the VALUES clause expanded to
// this set of arguments, until the arg stream has been processed.
// The queries are executed in a separate goroutine with a weighting of 1
// and can be executed concurrently to the extent allowed by the semaphore passed in sem.
// Entities for which the query ran successfully will be streamed on the succeeded channel.
func (db *DB) NamedBulkExec(
	ctx context.Context, query string, count int, sem *semaphore.Weighted,
	arg <-chan contracts.Entity, succeeded chan<- contracts.Entity, splitPolicyFactory com.BulkChunkSplitPolicyFactory,
) error {
	var counter com.Counter
	defer db.log(ctx, query, &counter).Stop()

	g, ctx := errgroup.WithContext(ctx)
	bulk := com.BulkEntities(ctx, arg, count, splitPolicyFactory)

	g.Go(func() error {
		for {
			select {
			case b, ok := <-bulk:
				if !ok {
					return nil
				}

				if err := sem.Acquire(ctx, 1); err != nil {
					return errors.Wrap(err, "can't acquire semaphore")
				}

				g.Go(func(b []contracts.Entity) func() error {
					return func() error {
						defer sem.Release(1)

						return retry.WithBackoff(
							ctx,
							func(ctx context.Context) error {
								_, err := db.NamedExecContext(ctx, query, b)
								if err != nil {
									return internal.CantPerformQuery(err, query)
								}

								counter.Add(uint64(len(b)))

								if succeeded != nil {
									for _, row := range b {
										select {
										case <-ctx.Done():
											return ctx.Err()
										case succeeded <- row:
										}
									}
								}

								return nil
							},
							IsRetryable,
							backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second),
							retry.Settings{},
						)
					}
				}(b))
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return g.Wait()
}

// NamedBulkExecTx bulk executes queries with named placeholders in separate transactions.
// Takes in up to the number of entities specified in count from the arg stream and
// executes a new transaction that runs a new query for each entity in this set of arguments,
// until the arg stream has been processed.
// The transactions are executed in a separate goroutine with a weighting of 1
// and can be executed concurrently to the extent allowed by the semaphore passed in sem.
func (db *DB) NamedBulkExecTx(
	ctx context.Context, query string, count int, sem *semaphore.Weighted, arg <-chan contracts.Entity,
) error {
	var counter com.Counter
	defer db.log(ctx, query, &counter).Stop()

	g, ctx := errgroup.WithContext(ctx)
	bulk := com.BulkEntities(ctx, arg, count, com.NeverSplit)

	g.Go(func() error {
		for {
			select {
			case b, ok := <-bulk:
				if !ok {
					return nil
				}

				if err := sem.Acquire(ctx, 1); err != nil {
					return errors.Wrap(err, "can't acquire semaphore")
				}

				g.Go(func(b []contracts.Entity) func() error {
					return func() error {
						defer sem.Release(1)

						return retry.WithBackoff(
							ctx,
							func(ctx context.Context) error {
								tx, err := db.BeginTxx(ctx, nil)
								if err != nil {
									return errors.Wrap(err, "can't start transaction")
								}

								stmt, err := tx.PrepareNamedContext(ctx, query)
								if err != nil {
									return errors.Wrap(err, "can't prepare named statement with context in transaction")
								}

								for _, arg := range b {
									if _, err := stmt.ExecContext(ctx, arg); err != nil {
										return errors.Wrap(err, "can't execute statement in transaction")
									}
								}

								if err := tx.Commit(); err != nil {
									return errors.Wrap(err, "can't commit transaction")
								}

								counter.Add(uint64(len(b)))

								return nil
							},
							IsRetryable,
							backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second),
							retry.Settings{},
						)
					}
				}(b))
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return g.Wait()
}

// BatchSizeByPlaceholders returns how often the specified number of placeholders fits
// into Options.MaxPlaceholdersPerStatement, but at least 1.
func (db *DB) BatchSizeByPlaceholders(n int) int {
	s := db.Options.MaxPlaceholdersPerStatement / n
	if s > 0 {
		return s
	}

	return 1
}

// YieldAll executes the query with the supplied scope,
// scans each resulting row into an entity returned by the factory function,
// and streams them into a returned channel.
func (db *DB) YieldAll(ctx context.Context, factoryFunc contracts.EntityFactoryFunc, query string, scope interface{}) (<-chan contracts.Entity, <-chan error) {
	entities := make(chan contracts.Entity, 1)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var counter com.Counter
		defer db.log(ctx, query, &counter).Stop()
		defer close(entities)

		rows, err := db.NamedQueryContext(ctx, query, scope)
		if err != nil {
			return internal.CantPerformQuery(err, query)
		}
		defer rows.Close()

		for rows.Next() {
			e := factoryFunc()

			if err := rows.StructScan(e); err != nil {
				return errors.Wrapf(err, "can't store query result into a %T: %s", e, query)
			}

			select {
			case entities <- e:
				counter.Inc()
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	})

	return entities, com.WaitAsync(g)
}

// CreateStreamed bulk creates the specified entities via NamedBulkExec.
// The insert statement is created using BuildInsertStmt with the first entity from the entities stream.
// Bulk size is controlled via Options.MaxPlaceholdersPerStatement and
// concurrency is controlled via Options.MaxConnectionsPerTable.
func (db *DB) CreateStreamed(ctx context.Context, entities <-chan contracts.Entity) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if first == nil {
		return errors.Wrap(err, "can't copy first entity")
	}

	sem := db.GetSemaphoreForTable(utils.TableName(first))
	stmt, placeholders := db.BuildInsertStmt(first)

	return db.NamedBulkExec(ctx, stmt, db.BatchSizeByPlaceholders(placeholders), sem, forward, nil, com.NeverSplit)
}

// UpsertStreamed bulk upserts the specified entities via NamedBulkExec.
// The upsert statement is created using BuildUpsertStmt with the first entity from the entities stream.
// Bulk size is controlled via Options.MaxPlaceholdersPerStatement and
// concurrency is controlled via Options.MaxConnectionsPerTable.
func (db *DB) UpsertStreamed(ctx context.Context, entities <-chan contracts.Entity, succeeded chan<- contracts.Entity) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if first == nil {
		return errors.Wrap(err, "can't copy first entity")
	}

	sem := db.GetSemaphoreForTable(utils.TableName(first))
	stmt, placeholders := db.BuildUpsertStmt(first)

	return db.NamedBulkExec(
		ctx, stmt, db.BatchSizeByPlaceholders(placeholders), sem, forward, succeeded, com.SplitOnDupId,
	)
}

// UpdateStreamed bulk updates the specified entities via NamedBulkExecTx.
// The update statement is created using BuildUpdateStmt with the first entity from the entities stream.
// Bulk size is controlled via Options.MaxRowsPerTransaction and
// concurrency is controlled via Options.MaxConnectionsPerTable.
func (db *DB) UpdateStreamed(ctx context.Context, entities <-chan contracts.Entity) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if first == nil {
		return errors.Wrap(err, "can't copy first entity")
	}
	sem := db.GetSemaphoreForTable(utils.TableName(first))
	stmt, _ := db.BuildUpdateStmt(first)

	return db.NamedBulkExecTx(ctx, stmt, db.Options.MaxRowsPerTransaction, sem, forward)
}

// DeleteStreamed bulk deletes the specified ids via BulkExec.
// The delete statement is created using BuildDeleteStmt with the passed entityType.
// Bulk size is controlled via Options.MaxPlaceholdersPerStatement and
// concurrency is controlled via Options.MaxConnectionsPerTable.
// IDs for which the query ran successfully will be streamed on the succeeded channel.
func (db *DB) DeleteStreamed(ctx context.Context, entityType contracts.Entity, ids <-chan interface{}, succeeded chan<- interface{}) error {
	sem := db.GetSemaphoreForTable(utils.TableName(entityType))
	return db.BulkExec(ctx, db.BuildDeleteStmt(entityType), db.Options.MaxPlaceholdersPerStatement, sem, ids, succeeded)
}

// Delete creates a channel from the specified ids and
// bulk deletes them by passing the channel along with the entityType to DeleteStreamed.
func (db *DB) Delete(ctx context.Context, entityType contracts.Entity, ids []interface{}) error {
	idsCh := make(chan interface{}, len(ids))
	for _, id := range ids {
		idsCh <- id
	}
	close(idsCh)

	return db.DeleteStreamed(ctx, entityType, idsCh, nil)
}

func (db *DB) GetSemaphoreForTable(table string) *semaphore.Weighted {
	db.tableSemaphoresMu.Lock()
	defer db.tableSemaphoresMu.Unlock()

	if sem, ok := db.tableSemaphores[table]; ok {
		return sem
	} else {
		sem = semaphore.NewWeighted(int64(db.Options.MaxConnectionsPerTable))
		db.tableSemaphores[table] = sem
		return sem
	}
}

func (db *DB) log(ctx context.Context, query string, counter *com.Counter) periodic.Stopper {
	return periodic.Start(ctx, db.logger.Interval(), func(tick periodic.Tick) {
		if count := counter.Reset(); count > 0 {
			db.logger.Debugf("Executed %q with %d rows", query, count)
		}
	}, periodic.OnStop(func(tick periodic.Tick) {
		db.logger.Debugf("Finished executing %q with %d rows in %s", query, counter.Total(), tick.Elapsed)
	}))
}

// IsRetryable checks whether the given error is retryable.
func IsRetryable(err error) bool {
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}

	if errors.Is(err, mysql.ErrInvalidConn) {
		return true
	}

	var e *mysql.MySQLError
	if errors.As(err, &e) {
		switch e.Number {
		case 1053, 1205, 1213, 2006:
			// 1053: Server shutdown in progress
			// 1205: Lock wait timeout
			// 1213: Deadlock found when trying to get lock
			// 2006: MySQL server has gone away
			return true
		default:
			return false
		}
	}

	var pe *pq.Error
	if errors.As(err, &pe) {
		switch pe.Code {
		case "08000", // connection_exception
			"08006", // connection_failure
			"08001", // sqlclient_unable_to_establish_sqlconnection
			"08004", // sqlserver_rejected_establishment_of_sqlconnection
			"40001", // serialization_failure
			"40P01", // deadlock_detected
			"54000", // program_limit_exceeded
			"55006", // object_in_use
			"55P03", // lock_not_available
			"57P01", // admin_shutdown
			"57P02", // crash_shutdown
			"57P03", // cannot_connect_now
			"58000", // system_error
			"58030", // io_error
			"XX000": // internal_error
			return true
		default:
			if strings.HasPrefix(string(pe.Code), "53") {
				// Class 53 - Insufficient Resources
				return true
			}
		}
	}

	return false
}
