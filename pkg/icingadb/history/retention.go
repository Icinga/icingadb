package history

import (
	"context"
	"fmt"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/periodic"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

// RetentionStatements maps history categories with corresponding cleanup statements.
var RetentionStatements = map[string]icingadb.CleanupStmt{
	"acknowledgement": {
		Table:  "acknowledgement_history",
		PK:     "id",
		Column: "clear_time",
	},
	"comment": {
		Table:  "comment_history",
		PK:     "comment_id",
		Column: "remove_time",
	},
	"downtime": {
		Table:  "downtime_history",
		PK:     "downtime_id",
		Column: "end_time",
	},
	"flapping": {
		Table:  "flapping_history",
		PK:     "id",
		Column: "end_time",
	},
	"notification": {
		Table:  "notification_history",
		PK:     "id",
		Column: "send_time",
	},
	"state": {
		Table:  "state_history",
		PK:     "id",
		Column: "event_time",
	},
}

// RetentionOptions defines the non-default mapping of history categories with their retention period in days.
type RetentionOptions map[string]uint64

// Validate checks constraints in the supplied retention options and
// returns an error if they are violated.
func (o RetentionOptions) Validate() error {
	for category := range o {
		if _, ok := RetentionStatements[category]; !ok {
			return errors.Errorf("invalid key %s for history retention", category)
		}
	}

	return nil
}

// Retention deletes rows from history tables that exceed their configured retention period.
type Retention struct {
	db       *icingadb.DB
	logger   *logging.Logger
	days     uint64
	interval time.Duration
	count    uint64
	options  RetentionOptions
}

// NewRetention returns a new Retention.
func NewRetention(db *icingadb.DB, days uint64, interval time.Duration, count uint64, options RetentionOptions, logger *logging.Logger) *Retention {
	return &Retention{
		db:       db,
		logger:   logger,
		days:     days,
		interval: interval,
		count:    count,
		options:  options,
	}
}

// Start starts the retention.
func (r *Retention) Start(ctx context.Context) error {
	return r.StartWithCallback(ctx, nil)
}

// StartWithCallback starts retention and executes the specified callback each time a retention run completes.
func (r *Retention) StartWithCallback(ctx context.Context, c func(table string, rs icingadb.CleanupResult)) error {
	ctx, cancelCtx := context.WithCancel(ctx)
	defer cancelCtx()

	errs := make(chan error, 1)

	for category, stmt := range RetentionStatements {
		days, ok := r.options[category]
		if !ok {
			days = r.days
		}

		if days < 1 {
			r.logger.Debugf("Skipping history retention for category %s", category)
			continue
		}

		r.logger.Debugw(
			fmt.Sprintf("Starting history retention for category %s", category),
			zap.Uint64("count", r.count),
			zap.Duration("interval", r.interval),
			zap.Uint64("retention-days", days),
		)

		category := category
		stmt := stmt
		periodic.Start(ctx, r.interval, func(tick periodic.Tick) {
			olderThan := tick.Time.AddDate(0, 0, -int(days))

			r.logger.Debugf("Cleaning up historical data for category %s older than %s", category, olderThan)

			rs, err := r.db.CleanupOlderThan(ctx, stmt, r.count, olderThan)
			if err != nil {
				select {
				case errs <- err:
				case <-ctx.Done():
				}

				return
			}

			if c != nil {
				c(stmt.Table, rs)
			}
		}, periodic.Immediate())
	}

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
