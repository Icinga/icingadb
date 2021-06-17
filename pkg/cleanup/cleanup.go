package cleanup

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/utils"
	"go.uber.org/zap"
	"sync"
	"time"
)

// Tables type defines mapping of history tables with corresponding period.
type Tables map[string]string

type Options struct {
	Interval time.Duration `yaml:"interval"  default:"1h"`
	Count    uint          `yaml:"count"     default:"5000"`
}

// tableconfig is defined for table configuration, with table name, period and start time.
type tableconfig struct {
	Table     string
	Period    time.Duration
	Starttime time.Time
}

// Cleanup is a cleanup wrapper with icingadb.DB wrapper and,
// logging and wait group synchronization capabilities.
type Cleanup struct {
	HistoryTables Tables
	logger        *zap.SugaredLogger
	db            *icingadb.DB
	wg            *sync.WaitGroup
	opts          *Options
}

// NewCleanup returns a new cleanup.Cleanup wrapper for the configured history tables.
func NewCleanup(db *icingadb.DB, tables Tables, opts *Options, logger *zap.SugaredLogger) *Cleanup {
	return &Cleanup{
		HistoryTables: tables,
		logger:        logger,
		db:            db,
		wg:            &sync.WaitGroup{},
		opts:          opts,
	}
}

// cleanupFunc type is declared to define cleanup functions signature.
type cleanupFunc func(ctx context.Context, db *icingadb.DB, tbl tableconfig, limit uint) (sql.Result, error)

// cleanupFuncs type is declared to define mapping of history tables with their corresponding cleanupFunc.
type cleanupFuncs map[string]cleanupFunc

// Start starts the cleanup routine for the configured history tables.
func (c *Cleanup) Start() error {
	if c.HistoryTables != nil {
		// cufuncConfig maps the history tables to corresponding cleanup methods.
		var cufuncConfig = func() cleanupFuncs {
			funcConfig := make(cleanupFuncs)

			for table, cufunc := range map[string]cleanupFunc{
				"acknowledgement": cleanAckFunc,
				"comment":         cleanCommentFunc,
				"downtime":        cleanDowntimeFunc,
				"flapping":        cleanFlappingFunc,
				"notification":    cleanNotificationFunc,
				"state":           cleanStateFunc,
			} {
				funcConfig[table] = cufunc
			}
			return funcConfig
		}()

		ctx := context.Background()
		for table, period := range c.HistoryTables {
			tempPeriod, _ := time.ParseDuration(period)
			c.wg.Add(1)
			go c.controller(ctx, tableconfig{table + "_history", tempPeriod, time.Now()}, cufuncConfig[table])
		}

		c.wg.Wait()
	}

	return nil
}

// controller calls the cleanup method periodically after c.opts.Interval.
func (c *Cleanup) controller(ctx context.Context, tbl tableconfig, cufunc cleanupFunc) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Calling NewTicker method
	d := time.NewTicker(c.opts.Interval)

	for {
		select {
		case <-d.C:
			// call cleanup method for the table
			_, err := c.cleanup(ctx, tbl, cufunc)
			if err != nil {
				c.logger.Error(err)

				cancel()
			}
		case <-ctx.Done():
			c.wg.Done()
			return
		}
	}
}

// cleanup calls the clean up method for the corresponding history table.
func (c *Cleanup) cleanup(ctx context.Context, tbl tableconfig, cufunc cleanupFunc) (int, error) {
	redo := true

	rowsAffected := make(chan int64, 1)
	defer close(rowsAffected)

	errs := make(chan error, 1)
	defer close(errs)
	limit := c.opts.Count

	tName := tbl.Table // table name.

	// numdel counts number of times delete statement executed.
	numdel := 0
	for redo {
		go func() {
			result, err := cufunc(ctx, c.db, tbl, limit)

			if err != nil {
				errs <- err
				return
			}
			affected, err := result.RowsAffected()

			if err != nil {
				errs <- err
				return
			}
			c.logger.Infof("Rows affected in %s: %v", tName, affected)
			rowsAffected <- affected
		}()

		select {
		case affected := <-rowsAffected:
			numdel++
			if affected < int64(limit) {
				redo = false
			}
			limit = c.opts.Count
		case <-time.After(time.Second): // if time taken to delete exceeds 1 second then limit is halved.
			limit = limit / 2
		case err := <-errs:
			return numdel, err
		}
	}
	return numdel, nil
}

// cleanAckFunc deletes records from "acknowledgement_history" table,
// where set_time < tblcfg.Starttime.Add(-1*tblcfg.Period)) and limit to c.opts.Count.
func cleanAckFunc(ctx context.Context, db *icingadb.DB, tblcfg tableconfig, limit uint) (sql.Result, error) {
	eventtime := utils.UnixMilli(tblcfg.Starttime.Add(-1 * tblcfg.Period))

	result, err := db.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE set_time < ? AND (clear_time IS NOT NULL AND clear_time < ?) LIMIT %d`, tblcfg.Table, limit),
		eventtime, eventtime)

	return result, err
}

// cleanCommentFunc deletes records from "comment_history" table,
// where entry_time < tblcfg.Starttime.Add(-1*tblcfg.Period)) and limit to c.opts.Count.
func cleanCommentFunc(ctx context.Context, db *icingadb.DB, tblcfg tableconfig, limit uint) (sql.Result, error) {
	eventtime := utils.UnixMilli(tblcfg.Starttime.Add(-1 * tblcfg.Period))
	result, err := db.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE entry_time < ? AND (remove_time IS NOT NULL AND remove_time < ?) LIMIT %d`, tblcfg.Table, limit),
		eventtime, eventtime)

	return result, err
}

// cleanDowntimeFunc deletes records from "downtime_history" table,
// where start_time < tblcfg.Starttime.Add(-1*tblcfg.Period)) and limit to c.opts.Count.
func cleanDowntimeFunc(ctx context.Context, db *icingadb.DB, tblcfg tableconfig, limit uint) (sql.Result, error) {
	eventtime := utils.UnixMilli(tblcfg.Starttime.Add(-1 * tblcfg.Period))

	result, err := db.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE start_time < ? AND (end_time IS NOT NULL AND end_time < ?) LIMIT %d`, tblcfg.Table, limit),
		eventtime, eventtime)

	return result, err
}

// cleanFlappingFunc deletes records from "flapping_history" table,
// where start_time < tblcfg.Starttime.Add(-1*tblcfg.Period)) and limit to c.opts.Count.
func cleanFlappingFunc(ctx context.Context, db *icingadb.DB, tblcfg tableconfig, limit uint) (sql.Result, error) {
	eventtime := utils.UnixMilli(tblcfg.Starttime.Add(-1 * tblcfg.Period))

	result, err := db.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE start_time < ? AND (end_time IS NOT NULL AND end_time < ?) LIMIT %d`, tblcfg.Table, limit),
		eventtime, eventtime)

	return result, err
}

// cleanNotificationFunc deletes records from "notification_history" table,
// where send_time < tblcfg.Starttime.Add(-1*tblcfg.Period)) and limit to c.opts.Count.
func cleanNotificationFunc(ctx context.Context, db *icingadb.DB, tblcfg tableconfig, limit uint) (sql.Result, error) {
	result, err := db.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE send_time < ? LIMIT %d`, tblcfg.Table, limit),
		utils.UnixMilli(tblcfg.Starttime.Add(-1*tblcfg.Period)))

	return result, err
}

// cleanStateFunc deletes records from "state_history" table,
// where event_time < tblcfg.Starttime.Add(-1*tblcfg.Period)) and limit to c.opts.Count.
func cleanStateFunc(ctx context.Context, db *icingadb.DB, tblcfg tableconfig, limit uint) (sql.Result, error) {
	result, err := db.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE event_time < ? LIMIT %d`, tblcfg.Table, limit),
		utils.UnixMilli(tblcfg.Starttime.Add(-1*tblcfg.Period)))

	return result, err
}
