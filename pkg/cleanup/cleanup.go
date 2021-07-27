package cleanup

import (
	"context"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"strconv"
	"time"
)

// HistoryRetention type defines mapping of history tables with corresponding retention period (days).
type HistoryRetention map[string]uint

type Options struct {
	Interval time.Duration `yaml:"interval"  default:"1h"`
	Count    uint          `yaml:"count"     default:"5000"`
}

// Cleanup is a cleanup wrapper with icingadb.DB wrapper and,
// logging and wait group synchronization capabilities.
type Cleanup struct {
	logger  *zap.SugaredLogger
	db      *icingadb.DB
	options Options
}

// NewCleanup returns a new cleanup.Cleanup wrapper for the configured history tables.
func NewCleanup(db *icingadb.DB, opts Options, logger *zap.SugaredLogger) *Cleanup {
	return &Cleanup{
		logger:  logger,
		db:      db,
		options: opts,
	}
}

type where struct {
	Time types.UnixMilli
}

// History starts the history cleanup routine for the configured history tables.
func (c *Cleanup) History(ctx context.Context, tables HistoryRetention) error {
	if tables != nil {
		// statements map tables to corresponding delete queries.
		var statements = map[string]string{
			"acknowledgement": "DELETE FROM acknowledgement_history WHERE set_time < :time AND (clear_time IS NOT NULL AND clear_time < :time)",
			"comment":         "DELETE FROM comment_history WHERE entry_time < :time AND (remove_time IS NOT NULL AND remove_time < :time)",
			"downtime":        "DELETE FROM downtime_history WHERE start_time < :time AND (end_time IS NOT NULL AND end_time < :time)",
			"flapping":        "DELETE FROM flapping_history WHERE start_time < :time AND (end_time IS NOT NULL AND end_time < :time)",
			"notification":    "DELETE FROM notification_history WHERE send_time < :time",
			"state":           "DELETE FROM state_history WHERE event_time < :time",
		}

		g, ctx := errgroup.WithContext(ctx)
		for table, period := range tables {
			g.Go(func() error {
				cancel, errcchan := c.controller(ctx, table, period, statements[table])
				defer cancel()

				select {
				case err := <-errcchan:
					return err
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		}

		return g.Wait()
	}

	c.logger.Warn("Cleanup is not run as no history tables are configured to cleanup.")
	return nil
}

// controller calls the cleanup method periodically after c.options.Interval.
func (c *Cleanup) controller(ctx context.Context, table string, period uint, statement string) (context.CancelFunc, <-chan error) {
	ctx, cancel := context.WithCancel(ctx)

	g, ctx := errgroup.WithContext(ctx)
	// Calling NewTicker method.
	ticker := time.NewTicker(c.options.Interval)

	g.Go(func() error {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// call cleanup method for the table.
				if _, err := c.cleanup(ctx, table, period, statement); err != nil {
					return err
				}

				ticker.Reset(c.options.Interval)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return cancel, com.WaitAsync(g)
}

// cleanup calls the clean up method for the corresponding history table.
func (c *Cleanup) cleanup(ctx context.Context, table string, period uint, statement string) (int, error) {
	redo := true
	cuCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cleanTime := time.Now().AddDate(0, 0, int(-period))
	rowsAffected := make(chan int64, 1)
	defer close(rowsAffected)

	errs := make(chan error, 1)
	defer close(errs)
	limit := c.options.Count

	// numdel counts number of times delete statement executed.
	numdel := 0
	for redo {
		go func() {
			result, err := c.db.NamedExecContext(cuCtx, statement+" LIMIT "+strconv.Itoa(int(limit)), &where{Time: types.UnixMilli(cleanTime)})

			if err != nil {
				errs <- err
				return
			}
			affected, err := result.RowsAffected()

			if err != nil {
				errs <- err
				return
			}
			c.logger.Infof("Rows affected in %s: %v", table, affected)
			rowsAffected <- affected
		}()

		select {
		case affected := <-rowsAffected:
			numdel++
			if affected < int64(limit) {
				redo = false
			}
			limit = c.options.Count
		case <-time.After(time.Second): // if time taken to delete exceeds 1 second then limit is halved.
			cancel()
			c.logger.Warnf("Time taken to cleanup %s exceeded 1 second; the number of records to cleanup is halved.", table)
			limit = limit / 2
			cuCtx, cancel = context.WithCancel(ctx)
			defer cancel()
		case err := <-errs:
			return numdel, err
		}
	}
	return numdel, nil
}
