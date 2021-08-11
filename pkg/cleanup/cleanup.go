package cleanup

import (
	"context"
	"fmt"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
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

// History starts the history cleanup routine for the configured history tables.
func (c *Cleanup) History(ctx context.Context, tables HistoryRetention) error {
	g, ctx := errgroup.WithContext(ctx)
	for table, period := range tables {
		stmt, ok := statements[table]
		if !ok {
			return errors.Errorf("invalid table %s for history retention", table)
		}

		g.Go(func() error {
			cancel, errs := utils.StablePeriodic(ctx, c.options.Interval, func(ctx context.Context, time time.Time) error {
				return c.cleanup(ctx, stmt, time.AddDate(0, 0, -int(period)))
			})
			defer cancel()

			select {
			case err := <-errs:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}

	return g.Wait()
}

// cleanup calls the clean up method for the corresponding history table.
func (c *Cleanup) cleanup(ctx context.Context, statement string, olderThan time.Time) error {
	r := &result{}
	defer utils.Timed(time.Now(), func(elapsed time.Duration) {
		c.logger.Debugf("Finished %s with %d rows and %d rounds in %s", statement, r.cnt.Val(), r.rounds.Val(), elapsed)
	})
	cancel := utils.Periodic(ctx, time.Second*10, func(elapsed time.Duration) {
		c.logger.Debugf("Executed %s with %d rows and %d rounds in %s", statement, r.cnt.Val(), r.rounds.Val(), elapsed)
	})
	defer cancel()

	affectedRows := make(chan int64, 1)
	defer close(affectedRows)

	for limit, stop := c.options.Count, false; !stop; {
		deleteCtx, cancelDeleteCtx := context.WithCancel(ctx)
		g, ctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			q := fmt.Sprintf("%s LIMIT %d", statement, limit)
			rs, err := c.db.NamedExecContext(deleteCtx, q, &where{Time: types.UnixMilli(olderThan)})
			if err != nil {
				if !utils.IsContextCanceled(err) {
					return internal.CantPerformQuery(err, q)
				}
				// Returns nil if only deleteCtx was canceled.
				return ctx.Err()
			}

			n, err := rs.RowsAffected()
			if err != nil {
				return err
			}

			select {
			case affectedRows <- n:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		g.Go(func() error {
			select {
			case n := <-affectedRows:
				r.rounds.Inc()
				r.cnt.Add(uint64(n))

				if n < int64(limit) {
					stop = true
				} else {
					limit = c.options.Count
				}
			case <-time.After(time.Second):
				limit = limit / 2
				c.logger.Warnf("Time taken to execute \"%s\" exceeded 1 second. Hence the limit is halved and the delete statement is re-executed with the new limit")
				cancelDeleteCtx()
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		})

		if err := g.Wait(); err != nil {
			return err
		}
	}

	return nil
}

// statements map tables to corresponding delete queries.
var statements = map[string]string{
	"acknowledgement": "DELETE FROM acknowledgement_history WHERE set_time < :time AND (clear_time IS NOT NULL AND clear_time < :time)",
	"comment":         "DELETE FROM comment_history WHERE entry_time < :time AND (remove_time IS NOT NULL AND remove_time < :time)",
	"downtime":        "DELETE FROM downtime_history WHERE start_time < :time AND (end_time IS NOT NULL AND end_time < :time)",
	"flapping":        "DELETE FROM flapping_history WHERE start_time < :time AND (end_time IS NOT NULL AND end_time < :time)",
	"notification":    "DELETE FROM notification_history WHERE send_time < :time",
	"state":           "DELETE FROM state_history WHERE event_time < :time",
}

type result struct {
	cnt    com.Counter
	rounds com.Counter
}

type where struct {
	Time types.UnixMilli
}
