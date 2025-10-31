package history

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icingadb/pkg/icingadb"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis/telemetry"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"strconv"
	"strings"
	"time"
)

type RetentionType int

const (
	RetentionHistory RetentionType = iota
	RetentionSla
)

type retentionStatement struct {
	icingadb.CleanupStmt
	RetentionType
	Category string
}

// RetentionStatements maps history categories with corresponding cleanup statements.
var RetentionStatements = []retentionStatement{{
	RetentionType: RetentionHistory,
	Category:      "acknowledgement",
	CleanupStmt: icingadb.CleanupStmt{
		Table:  "acknowledgement_history",
		PK:     "id",
		Column: "clear_time",
	},
}, {
	RetentionType: RetentionHistory,
	Category:      "comment",
	CleanupStmt: icingadb.CleanupStmt{
		Table:  "comment_history",
		PK:     "comment_id",
		Column: "remove_time",
	},
}, {
	RetentionType: RetentionHistory,
	Category:      "downtime",
	CleanupStmt: icingadb.CleanupStmt{
		Table:  "downtime_history",
		PK:     "downtime_id",
		Column: "end_time",
	},
}, {
	RetentionType: RetentionHistory,
	Category:      "flapping",
	CleanupStmt: icingadb.CleanupStmt{
		Table:  "flapping_history",
		PK:     "id",
		Column: "end_time",
	},
}, {
	RetentionType: RetentionHistory,
	Category:      "notification",
	CleanupStmt: icingadb.CleanupStmt{
		Table:  "notification_history",
		PK:     "id",
		Column: "send_time",
	},
}, {
	RetentionType: RetentionHistory,
	Category:      "state",
	CleanupStmt: icingadb.CleanupStmt{
		Table:  "state_history",
		PK:     "id",
		Column: "event_time",
	},
}, {
	RetentionType: RetentionSla,
	Category:      "sla_downtime",
	CleanupStmt: icingadb.CleanupStmt{
		Table:  "sla_history_downtime",
		PK:     "downtime_id",
		Column: "downtime_end",
	},
}, {
	RetentionType: RetentionSla,
	Category:      "sla_state",
	CleanupStmt: icingadb.CleanupStmt{
		Table:  "sla_history_state",
		PK:     "id",
		Column: "event_time",
	},
}}

// RetentionOptions defines the non-default mapping of history categories with their retention period in days.
type RetentionOptions map[string]uint16

// UnmarshalText implements [encoding.TextUnmarshaler] to allow RetentionOptions to be parsed by env.
//
// This custom TextUnmarshaler is necessary as - for the moment - env does not support map[T]encoding.TextUnmarshaler.
// After <https://github.com/caarlos0/env/pull/323> got merged and a new env release was drafted, this method can be
// removed.
func (o *RetentionOptions) UnmarshalText(text []byte) error {
	optionsMap := make(map[string]uint16)

	for pair := range strings.SplitSeq(string(text), ",") {
		key, value, found := strings.Cut(pair, ":")
		if !found {
			return fmt.Errorf("entry %q cannot be unmarshalled as a history-category:retention-period pair", pair)
		}

		days, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return fmt.Errorf("failed to parse %q as a uint16 retention period in days: %v", value, err)
		}

		optionsMap[key] = uint16(days)
	}

	*o = optionsMap

	return nil
}

// UnmarshalYAML implements yaml.InterfaceUnmarshaler to allow RetentionOptions to be parsed go-yaml.
func (o *RetentionOptions) UnmarshalYAML(unmarshal func(any) error) error {
	optionsMap := make(map[string]uint16)

	if err := unmarshal(&optionsMap); err != nil {
		return err
	}

	*o = optionsMap

	return nil
}

// Validate checks constraints in the supplied retention options and
// returns an error if they are violated.
func (o *RetentionOptions) Validate() error {
	allowedCategories := make(map[string]struct{})
	for _, stmt := range RetentionStatements {
		if stmt.RetentionType == RetentionHistory {
			allowedCategories[stmt.Category] = struct{}{}
		}
	}

	for category := range *o {
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
	historyDays uint16
	slaDays     uint16
	interval    time.Duration
	count       uint64
	options     RetentionOptions
}

// NewRetention returns a new Retention.
func NewRetention(
	db *database.DB, historyDays, slaDays uint16, interval time.Duration,
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
		var days uint16
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
			zap.Uint16("retention-days", days),
		)

		periodic.Start(ctx, r.interval, func(tick periodic.Tick) {
			olderThan := tick.Time.AddDate(0, 0, -int(days))

			r.logger.Debugf("Cleaning up historical data for category %s from table %s older than %s",
				stmt.Category, stmt.Table, olderThan)

			deleted, err := stmt.CleanupOlderThan(
				ctx, r.db, e.Id, r.count, olderThan,
				database.OnSuccessIncrement[struct{}](&telemetry.Stats.HistoryCleanup),
			)
			if err != nil {
				select {
				case errs <- err:
				case <-ctx.Done():
				}

				return
			}

			level := zap.DebugLevel
			if deleted > 0 {
				level = zap.InfoLevel
			}
			r.logger.Logf(level, "Removed %d old %s history items", deleted, stmt.Category)
		}, periodic.Immediate())
	}

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
