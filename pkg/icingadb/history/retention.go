package history

import (
	"context"
	"fmt"
	"github.com/icinga/icingadb/pkg/database"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis/telemetry"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/periodic"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

type RetentionType int

const (
	RetentionHistory RetentionType = iota
	RetentionSla
)

type retentionStatement struct {
	database.CleanupStmt
	RetentionType
	Category string
}

// RetentionStatements maps history categories with corresponding cleanup statements.
var RetentionStatements = []retentionStatement{{
	RetentionType: RetentionHistory,
	Category:      "acknowledgement",
	CleanupStmt: database.CleanupStmt{
		Table:  "acknowledgement_history",
		PK:     "id",
		Column: "clear_time",
	},
}, {
	RetentionType: RetentionHistory,
	Category:      "comment",
	CleanupStmt: database.CleanupStmt{
		Table:  "comment_history",
		PK:     "comment_id",
		Column: "remove_time",
	},
}, {
	RetentionType: RetentionHistory,
	Category:      "downtime",
	CleanupStmt: database.CleanupStmt{
		Table:  "downtime_history",
		PK:     "downtime_id",
		Column: "end_time",
	},
}, {
	RetentionType: RetentionHistory,
	Category:      "flapping",
	CleanupStmt: database.CleanupStmt{
		Table:  "flapping_history",
		PK:     "id",
		Column: "end_time",
	},
}, {
	RetentionType: RetentionHistory,
	Category:      "notification",
	CleanupStmt: database.CleanupStmt{
		Table:  "notification_history",
		PK:     "id",
		Column: "send_time",
	},
}, {
	RetentionType: RetentionHistory,
	Category:      "state",
	CleanupStmt: database.CleanupStmt{
		Table:  "state_history",
		PK:     "id",
		Column: "event_time",
	},
}, {
	RetentionType: RetentionSla,
	Category:      "sla_downtime",
	CleanupStmt: database.CleanupStmt{
		Table:  "sla_history_downtime",
		PK:     "downtime_id",
		Column: "downtime_end",
	},
}, {
	RetentionType: RetentionSla,
	Category:      "sla_state",
	CleanupStmt: database.CleanupStmt{
		Table:  "sla_history_state",
		PK:     "id",
		Column: "event_time",
	},
}}

// RetentionOptions defines the non-default mapping of history categories with their retention period in days.
type RetentionOptions map[string]uint64

// Validate checks constraints in the supplied retention options and
// returns an error if they are violated.
func (o RetentionOptions) Validate() error {
	allowedCategories := make(map[string]struct{})
	for _, stmt := range RetentionStatements {
		if stmt.RetentionType == RetentionHistory {
			allowedCategories[stmt.Category] = struct{}{}
		}
	}

	for category := range o {
		if _, ok := allowedCategories[category]; !ok {
			return errors.Errorf("invalid key %s for history retention", category)
		}
	}

	return nil
}

// Retention deletes rows from history tables that exceed their configured retention period.
type Retention struct {
	db          *database.DB
	logger      *logging.Logger
	historyDays uint64
	slaDays     uint64
	interval    time.Duration
	count       uint64
	options     RetentionOptions
}

// NewRetention returns a new Retention.
func NewRetention(
	db *database.DB, historyDays uint64, slaDays uint64, interval time.Duration,
	count uint64, options RetentionOptions, logger *logging.Logger,
) *Retention {
	return &Retention{
		db:          db,
		logger:      logger,
		historyDays: historyDays,
		slaDays:     slaDays,
		interval:    interval,
		count:       count,
		options:     options,
	}
}

// Start starts the retention.
func (r *Retention) Start(ctx context.Context) error {
	ctx, cancelCtx := context.WithCancel(ctx)
	defer cancelCtx()

	e, ok := v1.EnvironmentFromContext(ctx)
	if !ok {
		return errors.New("can't get environment from context")
	}

	errs := make(chan error, 1)

	for _, stmt := range RetentionStatements {
		var days uint64
		switch stmt.RetentionType {
		case RetentionHistory:
			if d, ok := r.options[stmt.Category]; ok {
				days = d
			} else {
				days = r.historyDays
			}
		case RetentionSla:
			days = r.slaDays
		}

		if days < 1 {
			r.logger.Debugf("Skipping history retention for category %s", stmt.Category)
			continue
		}

		r.logger.Debugw(
			fmt.Sprintf("Starting history retention for category %s", stmt.Category),
			zap.Uint64("count", r.count),
			zap.Duration("interval", r.interval),
			zap.Uint64("retention-days", days),
		)

		stmt := stmt
		periodic.Start(ctx, r.interval, func(tick periodic.Tick) {
			olderThan := tick.Time.AddDate(0, 0, -int(days))

			r.logger.Debugf("Cleaning up historical data for category %s from table %s older than %s",
				stmt.Category, stmt.Table, olderThan)

			deleted, err := r.db.CleanupOlderThan(
				ctx, stmt.CleanupStmt, e.Id, r.count, olderThan,
				database.OnSuccessIncrement[struct{}](&telemetry.Stats.HistoryCleanup),
			)
			if err != nil {
				select {
				case errs <- err:
				case <-ctx.Done():
				}

				return
			}

			if deleted > 0 {
				r.logger.Infof("Removed %d old %s history items", deleted, stmt.Category)
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
