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
    "time"
)

// DB is a wrapper around sqlx.DB with bulk execution,
// statement building, streaming and logging capabilities.
type DB struct {
    *sqlx.DB

    logger *zap.SugaredLogger
}

// NewDb returns a new icingadb.DB wrapper for a pre-existing *sqlx.DB.
func NewDb(db *sqlx.DB, logger *zap.SugaredLogger) *DB {
    return &DB{DB: db, logger: logger}
}

func (db DB) BuildColumns(subject interface{}) []string {
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

func (db DB) BuildDeleteStmt(from interface{}) string {
    return fmt.Sprintf(
        `DELETE FROM %s WHERE id IN (?)`,
        utils.Key(utils.Name(from), '_'),
    )
}

func (db DB) BuildInsertStmt(into interface{}) string {
    columns := db.BuildColumns(into)

    return fmt.Sprintf(
        `INSERT INTO %s (%s) VALUES (%s)`,
        utils.Key(utils.Name(into), '_'),
        strings.Join(columns, ", "),
        fmt.Sprintf(":%s", strings.Join(columns, ", :")),
    )
}

func (db DB) BuildSelectStmt(from interface{}, into interface{}) string {
    return fmt.Sprintf(
        `SELECT %s FROM %s`,
        strings.Join(db.BuildColumns(into), ", "),
        utils.Key(utils.Name(from), '_'),
    )
}

func (db DB) BuildUpdateStmt(update interface{}) string {
    columns := db.BuildColumns(update)
    set := make([]string, 0, len(columns))

    for _, col := range columns {
        set = append(set, fmt.Sprintf("%s = :%s", col, col))
    }

    return fmt.Sprintf(
        `UPDATE %s SET %s WHERE id = :id`,
        utils.Key(utils.Name(update), '_'),
        strings.Join(set, ", "),
    )
}

func (db DB) BuildUpsertStmt(subject interface{}) string {
    columns := db.BuildColumns(subject)
    set := make([]string, 0, len(columns))

    for _, col := range columns {
        set = append(set, fmt.Sprintf("%s = :%s", col, col))
    }

    return fmt.Sprintf(
        `INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s`,
        utils.Key(utils.Name(subject), '_'),
        strings.Join(columns, ","),
        fmt.Sprintf(":%s", strings.Join(columns, ",:")),
        strings.Join(set, ","),
    )
}

func (db DB) BulkExec(ctx context.Context, query string, count int, concurrent int, args []interface{}) error {
    var cnt com.Counter
    g, ctx := errgroup.WithContext(ctx)
    // Use context from group.
    batches := utils.BatchSliceOfInterfaces(ctx, args, count)

    db.logger.Debugf("Executing %s", query)
    defer utils.Timed(time.Now(), func(elapsed time.Duration) {
        db.logger.Debugf("Executed %s with %d rows in %s", query, cnt.Val(), elapsed)
    })

    g.Go(func() error {
        sem := semaphore.NewWeighted(int64(concurrent))

        g, ctx := errgroup.WithContext(ctx)

        for b := range batches {
            if err := sem.Acquire(ctx, 1); err != nil {
                return err
            }

            g.Go(func(b []interface{}) func() error {
                return func() error {
                    defer sem.Release(1)

                    return retry.WithBackoff(
                        ctx,
                        func() error {
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
                        backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second))
                }
            }(b))
        }

        return g.Wait()
    })

    return g.Wait()
}

func (db DB) NamedBulkExec(ctx context.Context, query string, count int, concurrent int, arg chan interface{}) error {
    var cnt com.Counter
    g, ctx := errgroup.WithContext(ctx)
    bulk := com.Bulk(ctx, arg, count)

    db.logger.Debugf("Executing %s", query)
    defer utils.Timed(time.Now(), func(elapsed time.Duration) {
        db.logger.Debugf("Executed %s with %d rows in %s", query, cnt.Val(), elapsed)
    })

    g.Go(func() error {
        sem := semaphore.NewWeighted(int64(concurrent))
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

                g.Go(func(b []interface{}) func() error {
                    return func() error {
                        defer sem.Release(1)

                        return retry.WithBackoff(
                            ctx,
                            func() error {
                                db.logger.Debugf("Executing %s with %d rows..", query, len(b))
                                start := time.Now()
                                _, err := db.NamedExecContext(ctx, query, b)
                                if err != nil {
                                    fmt.Println(err)
                                    return err
                                }
                                db.logger.Debugf("..took %s", time.Since(start))

                                cnt.Add(uint64(len(b)))

                                return nil
                            },
                            IsRetryable,
                            backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second))
                    }
                }(b))
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    })

    return g.Wait()
}

func (db DB) NamedBulkExecTx(ctx context.Context, query string, count int, concurrent int, arg chan interface{}) error {
    var cnt com.Counter
    g, ctx := errgroup.WithContext(ctx)
    bulk := com.Bulk(ctx, arg, count)

    db.logger.Debugf("Executing %s", query)
    defer utils.Timed(time.Now(), func(elapsed time.Duration) {
        db.logger.Debugf("Executed %s with %d rows in %s", query, cnt.Val(), elapsed)
    })

    g.Go(func() error {
        sem := semaphore.NewWeighted(int64(concurrent))

        for {
            select {
            case b, ok := <-bulk:
                if !ok {
                    return nil
                }

                if err := sem.Acquire(ctx, 1); err != nil {
                    return err
                }

                g.Go(func(b []interface{}) func() error {
                    return func() error {
                        defer sem.Release(1)

                        return retry.WithBackoff(
                            ctx,
                            func() error {
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
                            backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second))
                    }
                }(b))
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    })

    return g.Wait()
}

func (db DB) YieldAll(ctx context.Context, factoryFunc contracts.EntityFactoryFunc, query string, args ...interface{}) (<-chan contracts.Entity, <-chan error) {
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

func (db DB) Create(ctx context.Context, entities <-chan contracts.Entity) error {
    // TODO(el): Check ctx.Done()?
    entity := <-entities
    if entity == nil {
        return nil
    }
    // Buffer of one because we receive an entity and send it back immediately.
    inserts := make(chan interface{}, 1)
    inserts <- entity

    go func() {
        defer close(inserts)

        for e := range entities {
            select {
            case inserts <- e:
            case <-ctx.Done():
                return
            }
        }
    }()

    return db.NamedBulkExec(ctx, db.BuildInsertStmt(entity), 1<<15/len(db.BuildColumns(entity)), 1<<3, inserts)
}

func (db DB) Update(ctx context.Context, entities <-chan contracts.Entity) error {
    // TODO(el): Check ctx.Done()?
    entity := <-entities
    if entity == nil {
        return nil
    }
    // Buffer of one because we receive an entity and send it back immediately.
    updates := make(chan interface{}, 1)
    updates <- entity

    go func() {
        defer close(updates)

        for e := range entities {
            select {
            case updates <- e:
            case <-ctx.Done():
                return
            }
        }
    }()

    return db.NamedBulkExecTx(ctx, db.BuildUpdateStmt(entity), 1<<15, 1<<3, updates)
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
