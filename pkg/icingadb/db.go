package icingadb

import (
	"context"
	"fmt"
	"github.com/go-sql-driver/mysql"
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

	logger            *zap.SugaredLogger
	options           *Options
	tableSemaphores   map[string]*semaphore.Weighted
	tableSemaphoresMu sync.Mutex
}

type Options struct {
	MaxConnections         int `yaml:"max_connections" default:"16"`
	MaxConnectionsPerTable int `yaml:"max_connections_per_table" default:"8"`
}

// NewDb returns a new icingadb.DB wrapper for a pre-existing *sqlx.DB.
func NewDb(db *sqlx.DB, logger *zap.SugaredLogger, options *Options) *DB {
	return &DB{
		DB:              db,
		logger:          logger,
		options:         options,
		tableSemaphores: make(map[string]*semaphore.Weighted),
	}
}

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

func (db *DB) BuildDeleteStmt(from interface{}) string {
	return fmt.Sprintf(
		`DELETE FROM %s WHERE id IN (?)`,
		utils.TableName(from),
	)
}

func (db *DB) BuildInsertStmt(into interface{}) string {
	columns := db.BuildColumns(into)

	return fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s)`,
		utils.TableName(into),
		strings.Join(columns, ", "),
		fmt.Sprintf(":%s", strings.Join(columns, ", :")),
	)
}

func (db *DB) BuildSelectStmt(from interface{}, into interface{}) string {
	return fmt.Sprintf(
		`SELECT %s FROM %s`,
		strings.Join(db.BuildColumns(into), ", "),
		utils.TableName(from),
	)
}

func (db *DB) BuildUpdateStmt(update interface{}) string {
	columns := db.BuildColumns(update)
	set := make([]string, 0, len(columns))

	for _, col := range columns {
		set = append(set, fmt.Sprintf("%s = :%s", col, col))
	}

	return fmt.Sprintf(
		`UPDATE %s SET %s WHERE id = :id`,
		utils.TableName(update),
		strings.Join(set, ", "),
	)
}

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
		set = append(set, fmt.Sprintf("%s = :%s", col, col))
	}

	return fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s`,
		utils.TableName(subject),
		strings.Join(insertColumns, ","),
		fmt.Sprintf(":%s", strings.Join(insertColumns, ",:")),
		strings.Join(set, ","),
	), len(insertColumns) + len(updateColumns)
}

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
				return err
			}

			g.Go(func(b []interface{}) func() error {
				return func() error {
					defer sem.Release(1)

					return retry.WithBackoff(
						ctx,
						func(context.Context) error {
							query, args, err := sqlx.In(query, b)
							if err != nil {
								return err
							}

							query = db.Rebind(query)
							_, err = db.Query(query, args...)
							if err != nil {
								return err
							}

							cnt.Add(uint64(len(b)))

							return nil
						},
						IsRetryable,
						backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second),
						0,
					)
				}
			}(b))
		}

		return g.Wait()
	})

	return g.Wait()
}

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
		// stmt, err := db.PrepareNamedContext(ctx, query)
		// if err != nil {
		//     return err
		// }

		for {
			select {
			case b, ok := <-bulk:
				if !ok {
					return nil
				}

				if err := sem.Acquire(ctx, 1); err != nil {
					return err
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
									fmt.Println(err)
									return err
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
							0,
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
					return err
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
							0,
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

		rows, err := db.Queryx(query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			e := factoryFunc()

			if err := rows.StructScan(e); err != nil {
				return err
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

func (db *DB) CreateStreamed(ctx context.Context, entities <-chan contracts.Entity) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if first == nil {
		return err
	}

	sem := db.getSemaphoreForTable(utils.TableName(first))
	return db.NamedBulkExec(ctx, db.BuildInsertStmt(first), 1<<15/len(db.BuildColumns(first)), sem, forward, nil)
}

func (db *DB) UpsertStreamed(ctx context.Context, entities <-chan contracts.Entity, succeeded chan<- contracts.Entity) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if first == nil {
		return err
	}

	// TODO(ak): wait for https://github.com/jmoiron/sqlx/issues/694
	//stmt, placeholders := db.BuildUpsertStmt(first)
	//return db.NamedBulkExec(ctx, stmt, 1<<15/placeholders, 1<<3, forward, succeeded)
	stmt, _ := db.BuildUpsertStmt(first)
	sem := db.getSemaphoreForTable(utils.TableName(first))
	return db.NamedBulkExec(ctx, stmt, 1, sem, forward, succeeded)
}

func (db *DB) UpdateStreamed(ctx context.Context, entities <-chan contracts.Entity) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if first == nil {
		return err
	}
	sem := db.getSemaphoreForTable(utils.TableName(first))
	return db.NamedBulkExecTx(ctx, db.BuildUpdateStmt(first), 1<<15, sem, forward)
}

func (db *DB) DeleteStreamed(ctx context.Context, entityType contracts.Entity, ids <-chan interface{}) error {
	sem := db.getSemaphoreForTable(utils.TableName(entityType))
	return db.BulkExec(ctx, db.BuildDeleteStmt(entityType), 1<<15, sem, ids)
}

func (db *DB) Delete(ctx context.Context, entityType contracts.Entity, ids []interface{}) error {
	idsCh := make(chan interface{}, len(ids))
	for _, id := range ids {
		idsCh <- id
	}
	close(idsCh)

	return db.DeleteStreamed(ctx, entityType, idsCh)
}

func IsRetryable(err error) bool {
	err = errors.Cause(err)

	if err == mysql.ErrInvalidConn {
		return true
	}

	switch e := err.(type) {
	case *mysql.MySQLError:
		switch e.Number {
		case 1053, 1205, 1213, 2006:
			// 1053: Server shutdown in progress
			// 1205:
			// 1213:
			// 2006: MySQL server has gone away
			return true
		}
	}

	return false
}

func (db *DB) getSemaphoreForTable(table string) *semaphore.Weighted {
	db.tableSemaphoresMu.Lock()
	defer db.tableSemaphoresMu.Unlock()

	if sem, ok := db.tableSemaphores[table]; ok {
		return sem
	} else {
		sem = semaphore.NewWeighted(int64(db.options.MaxConnectionsPerTable))
		db.tableSemaphores[table] = sem
		return sem
	}
}
