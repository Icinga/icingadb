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
	"github.com/icinga/icingadb/pkg/retry"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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

	logger            *zap.SugaredLogger
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

	return nil
}

// NewDb returns a new icingadb.DB wrapper for a pre-existing *sqlx.DB.
func NewDb(db *sqlx.DB, logger *zap.SugaredLogger, options *Options) *DB {
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
		`DELETE FROM %s WHERE id IN (?)`,
		utils.TableName(from),
	)
}

// BuildInsertStmt returns an INSERT INTO statement for the given struct.
func (db *DB) BuildInsertStmt(into interface{}) (string, int) {
	columns := db.BuildColumns(into)

	return fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s)`,
		utils.TableName(into),
		strings.Join(columns, ", "),
		fmt.Sprintf(":%s", strings.Join(columns, ", :")),
	), len(columns)
}

// BuildSelectStmt returns a SELECT query that creates the FROM part from the given table struct
// and the column list from the specified columns struct.
func (db *DB) BuildSelectStmt(table interface{}, columns interface{}) string {
	return fmt.Sprintf(
		`SELECT %s FROM %s`,
		strings.Join(db.BuildColumns(columns), ", "),
		utils.TableName(table),
	)
}

// BuildUpdateStmt returns an UPDATE statement for the given struct.
func (db *DB) BuildUpdateStmt(update interface{}) (string, int) {
	columns := db.BuildColumns(update)
	set := make([]string, 0, len(columns))

	for _, col := range columns {
		set = append(set, fmt.Sprintf("%s = :%s", col, col))
	}

	return fmt.Sprintf(
		`UPDATE %s SET %s WHERE id = :id`,
		utils.TableName(update),
		strings.Join(set, ", "),
	), len(columns) + 1 // +1 because of WHERE id = :id
}

// BuildUpsertStmt returns an upsert statement for the given struct.
func (db *DB) BuildUpsertStmt(subject interface{}) (stmt string, placeholders int) {
	insertColumns := db.BuildColumns(subject)
	var updateColumns []string

	if upserter, ok := subject.(contracts.Upserter); ok {
		updateColumns = db.BuildColumns(upserter.Upsert())
	} else {
		updateColumns = insertColumns
	}

	set := make([]string, 0, len(updateColumns))

	for _, col := range updateColumns {
		set = append(set, fmt.Sprintf("%s = VALUES(%s)", col, col))
	}

	return fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s`,
		utils.TableName(subject),
		strings.Join(insertColumns, ","),
		fmt.Sprintf(":%s", strings.Join(insertColumns, ",:")),
		strings.Join(set, ","),
	), len(insertColumns)
}

// BulkExec bulk executes queries with a single slice placeholder in the form of `IN (?)`.
// Takes in up to the number of arguments specified in count from the arg stream,
// derives and expands a query and executes it with this set of arguments until the arg stream has been processed.
// The derived queries are executed in a separate goroutine with a weighting of 1
// and can be executed concurrently to the extent allowed by the semaphore passed in sem.
func (db *DB) BulkExec(ctx context.Context, query string, count int, sem *semaphore.Weighted, arg <-chan interface{}) error {
	var cnt com.Counter
	g, ctx := errgroup.WithContext(ctx)
	// Use context from group.
	bulk := com.Bulk(ctx, arg, count)

	db.logger.Debugf("Executing %s", query)
	defer utils.Timed(time.Now(), func(elapsed time.Duration) {
		db.logger.Debugf("Executed %s with %d rows in %s", query, cnt.Val(), elapsed)
	})

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

							cnt.Add(uint64(len(b)))

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
	arg <-chan contracts.Entity, succeeded chan<- contracts.Entity,
) error {
	var cnt com.Counter
	g, ctx := errgroup.WithContext(ctx)
	bulk := com.BulkEntities(ctx, arg, count)

	db.logger.Debugf("Executing %s", query)
	defer utils.Timed(time.Now(), func(elapsed time.Duration) {
		db.logger.Debugf("Executed %s with %d rows in %s", query, cnt.Val(), elapsed)
	})

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
								db.logger.Debugf("Executing %s with %d rows..", query, len(b))
								_, err := db.NamedExecContext(ctx, query, b)
								if err != nil {
									return internal.CantPerformQuery(err, query)
								}

								cnt.Add(uint64(len(b)))

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
	var cnt com.Counter
	g, ctx := errgroup.WithContext(ctx)
	bulk := com.BulkEntities(ctx, arg, count)

	db.logger.Debugf("Executing %s", query)
	defer utils.Timed(time.Now(), func(elapsed time.Duration) {
		db.logger.Debugf("Executed %s with %d rows in %s", query, cnt.Val(), elapsed)
	})

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

								cnt.Add(uint64(len(b)))

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

// YieldAll executes the query with the supplied args,
// scans each resulting row into an entity returned by the factory function,
// and streams them into a returned channel.
func (db *DB) YieldAll(ctx context.Context, factoryFunc contracts.EntityFactoryFunc, query string, args ...interface{}) (<-chan contracts.Entity, <-chan error) {
	var cnt com.Counter
	entities := make(chan contracts.Entity, 1)
	g, ctx := errgroup.WithContext(ctx)

	db.logger.Infof("Syncing %s", query)

	g.Go(func() error {
		defer close(entities)
		defer utils.Timed(time.Now(), func(elapsed time.Duration) {
			v := factoryFunc()
			db.logger.Infof("Fetched %d elements of %s in %s", cnt.Val(), utils.Name(v), elapsed)
		})

		rows, err := db.QueryxContext(ctx, query, args...)
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
				cnt.Inc()
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

	return db.NamedBulkExec(ctx, stmt, db.BatchSizeByPlaceholders(placeholders), sem, forward, nil)
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

	return db.NamedBulkExec(ctx, stmt, db.BatchSizeByPlaceholders(placeholders), sem, forward, succeeded)
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
func (db *DB) DeleteStreamed(ctx context.Context, entityType contracts.Entity, ids <-chan interface{}) error {
	sem := db.GetSemaphoreForTable(utils.TableName(entityType))
	return db.BulkExec(ctx, db.BuildDeleteStmt(entityType), db.Options.MaxPlaceholdersPerStatement, sem, ids)
}

// Delete creates a channel from the specified ids and
// bulk deletes them by passing the channel along with the entityType to DeleteStreamed.
func (db *DB) Delete(ctx context.Context, entityType contracts.Entity, ids []interface{}) error {
	idsCh := make(chan interface{}, len(ids))
	for _, id := range ids {
		idsCh <- id
	}
	close(idsCh)

	return db.DeleteStreamed(ctx, entityType, idsCh)
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
		}
	}

	return false
}
