package icingadb

import (
	"context"
	"crypto/sha1" // #nosec G505 -- required by Icinga 2's HashValue function
	"database/sql"
	"fmt"
	"time"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/objectpacker"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icingadb/pkg/icingadb/v1/history"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// mkGenericHistoryMockStmts creates SQL queries to acquire data and to create a new history entry.
//
// This helper function is used in the methods implementing closeObjectAndMockHistory below.
func mkGenericHistoryMockStmts(tableName string) (stmtPopulate, stmtHistoryInsert string) {
	stmtPopulate = fmt.Sprintf(`
		SELECT
			history.environment_id AS environment_id,
			history.endpoint_id AS endpoint_id,
			history.object_type AS object_type,
			history.host_id AS host_id,
			history.service_id AS service_id,
			%[1]s.name AS name
		FROM %[1]s
		JOIN history ON
			%[1]s.id = history.%[1]s_history_id
		WHERE %[1]s.id = :%[1]s_id`, tableName)

	stmtHistoryInsert = fmt.Sprintf(`
		INSERT INTO history (
			id, environment_id, endpoint_id, object_type, host_id, service_id, %[1]s_history_id, event_type, event_time
		) VALUES (
			:id, :environment_id, :endpoint_id, :object_type, :host_id, :service_id, :%[1]s_id, :event_type, :event_time
		)`, tableName)

	return
}

// calcEventId mocks Icinga 2's IcingaDB::CalcEventID method for missing history entries.
//
// This is a helper function used below to simulate a new history.id primary key.
func calcEventId(environmentId, eventType, objectName string) (types.Binary, error) {
	h := sha1.New() // #nosec G401 -- required by Icinga 2's HashValue function

	err := objectpacker.PackAny([]string{environmentId, eventType, objectName}, h)
	if err != nil {
		return nil, err
	}

	return types.Binary(h.Sum(nil)), nil
}

// closeObjectAndMockHistory manually closes vanished Icinga 2 objects, e.g., Downtimes of vanished Hosts.
//
// This method provides a common stub for each type-specific implementation below.
func (s Sync) closeObjectAndMockHistory(
	ctx context.Context,
	ids []any,
	objectType string,
	action func(ctx context.Context, tx *sqlx.Tx, id types.Binary) error,
) error {
	s.logger.Debugw("Start manually closing vanished objects",
		zap.String("type", objectType),
		zap.Int("amount", len(ids)))

	binaryIds := make([]types.Binary, 0, len(ids))
	for _, id := range ids {
		binaryId, ok := id.(types.Binary)
		if !ok {
			return errors.Errorf("ID is not types.Binary, but %T", id)
		}
		binaryIds = append(binaryIds, binaryId)
	}

	// Process action callback in bulks of the given step size instead of creating a transaction for
	// each action anew. The current step size is quite arbitrary, tweaked during testing.
	const steps = 256
	for infimum := 0; infimum < len(binaryIds); infimum += steps {
		startTime := time.Now()

		err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) error {
				return s.db.ExecTx(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
					for i := infimum; i < min(len(binaryIds), infimum+steps); i++ {
						err := action(ctx, tx, binaryIds[i])
						if err != nil {
							return err
						}
					}

					return nil
				})
			},
			retry.Retryable,
			backoff.DefaultBackoff,
			s.db.GetDefaultRetrySettings())
		if err != nil {
			return errors.Wrap(err, "can't mock object end")
		}

		s.logger.Debugw("Manually closed vanished objects",
			zap.String("type", objectType),
			zap.Int("amount", min(len(binaryIds), infimum+steps)-infimum),
			zap.Duration("duration", time.Since(startTime)))
	}

	return nil
}

// mockCommentEnd manually closes comments by altering the related history tables.
//
// This affects history retention by setting comment_history.remove_time.
func (s Sync) mockCommentEnd(ctx context.Context, ids []any) error {
	type Comment struct {
		history.CommentHistory
		history.HistoryMeta

		Name      types.String
		EventTime types.UnixMilli
	}

	stmtPopulate, stmtHistoryInsert := mkGenericHistoryMockStmts("comment")

	// Marks the existing comment_history as removed.
	stmtCommentHistoryUpdate := `
		UPDATE comment_history
		SET
			removed_by = :removed_by,
			remove_time = :remove_time,
			has_been_removed = :has_been_removed
		WHERE comment_id = :comment_id`

	return s.closeObjectAndMockHistory(
		ctx,
		ids,
		"comment",
		func(ctx context.Context, tx *sqlx.Tx, id types.Binary) error {
			eventTime := time.Now()

			comment := &Comment{
				CommentHistory: history.CommentHistory{
					CommentHistoryEntity: history.CommentHistoryEntity{
						CommentId: id,
					},
				},
			}

			// Start by populating information based on the previous data.
			prepStmtCommentPopulate, err := tx.PrepareNamedContext(ctx, stmtPopulate)
			if err != nil {
				return errors.Wrap(err, "can't prepare statement to populate entry")
			}
			defer func() { _ = prepStmtCommentPopulate.Close() }()

			err = prepStmtCommentPopulate.GetContext(ctx, comment, comment)
			if errors.Is(err, sql.ErrNoRows) {
				s.logger.Infow("Can't fetch vanished Comment from database for cleanup",
					zap.String("comment_id", id.String()),
					zap.Error(err))
				return nil
			} else if err != nil {
				return errors.Wrap(err, "can't fetch information to populate entry")
			}

			// Fill fields to update comment_history
			comment.RemovedBy = types.MakeString("Comment Config Removed")
			comment.RemoveTime = types.UnixMilli(eventTime)
			comment.HasBeenRemoved = types.MakeBool(true)

			_, err = tx.NamedExecContext(ctx, stmtCommentHistoryUpdate, comment)
			if err != nil {
				return errors.Wrap(err, "can't update comment_history")
			}

			// Update fields for a new history entry.
			comment.Id, err = calcEventId(comment.EndpointId.String(), "comment_remove", comment.Name.String)
			if err != nil {
				return errors.Wrap(err, "can't calculate event_id")
			}

			comment.EventType = "comment_remove"
			comment.EventTime = types.UnixMilli(eventTime)

			_, err = tx.NamedExecContext(ctx, stmtHistoryInsert, comment)
			if err != nil {
				return errors.Wrap(err, "can't insert new history entry")
			}

			return nil
		})
}

// mockDowntimeEnd manually closes downtimes by altering the related history tables.
//
// This affects SLA calculations (sla_history_donwtime table).
func (s Sync) mockDowntimeEnd(ctx context.Context, ids []any) error {
	type Downtime struct {
		history.DowntimeHistory
		history.HistoryMeta

		Name      types.String
		EventTime types.UnixMilli
	}

	stmtPopulate, stmtHistoryInsert := mkGenericHistoryMockStmts("downtime")

	// Marks the existing downtime_history as cancelled.
	stmtDowntimeHistoryUpdate := `
		UPDATE downtime_history
		SET
			cancelled_by = :cancelled_by,
			has_been_cancelled = :has_been_cancelled,
			cancel_time = :cancel_time
		WHERE downtime_id = :downtime_id`

	// Mark the existing sla_history_downtime as ended.
	stmtSlaHistoryDowntimeUpdate := `
		UPDATE sla_history_downtime
		SET downtime_end = :cancel_time
		WHERE downtime_id = :downtime_id`

	return s.closeObjectAndMockHistory(
		ctx,
		ids,
		"downtime",
		func(ctx context.Context, tx *sqlx.Tx, id types.Binary) error {
			eventTime := time.Now()

			downtime := &Downtime{
				DowntimeHistory: history.DowntimeHistory{
					DowntimeHistoryEntity: history.DowntimeHistoryEntity{
						DowntimeId: id,
					},
				},
			}

			// Start by populating information based on the previous data.
			prepStmtHistorySelect, err := tx.PrepareNamedContext(ctx, stmtPopulate)
			if err != nil {
				return errors.Wrap(err, "can't prepare statement to populate entry")
			}
			defer func() { _ = prepStmtHistorySelect.Close() }()

			err = prepStmtHistorySelect.GetContext(ctx, downtime, downtime)
			if errors.Is(err, sql.ErrNoRows) {
				s.logger.Infow("Can't fetch vanished Downtime from database for cleanup",
					zap.String("downtime_id", id.String()),
					zap.Error(err))
				return nil
			} else if err != nil {
				return errors.Wrap(err, "can't fetch information to populate entry")
			}

			// Fill fields to update downtime_history and sla_history_downtime.
			downtime.CancelledBy = types.MakeString("Downtime Config Removed")
			downtime.HasBeenCancelled = types.MakeBool(true)
			downtime.CancelTime = types.UnixMilli(eventTime)

			_, err = tx.NamedExecContext(ctx, stmtDowntimeHistoryUpdate, downtime)
			if err != nil {
				return errors.Wrap(err, "can't update downtime_history")
			}

			_, err = tx.NamedExecContext(ctx, stmtSlaHistoryDowntimeUpdate, downtime)
			if err != nil {
				return errors.Wrap(err, "can't update sla_history_downtime")
			}

			// Update fields for a new history entry.
			downtime.Id, err = calcEventId(downtime.EnvironmentId.String(), "downtime_end", downtime.Name.String)
			if err != nil {
				return errors.Wrap(err, "can't calculate event_id")
			}

			downtime.EventType = "downtime_end"
			downtime.EventTime = types.UnixMilli(eventTime)

			_, err = tx.NamedExecContext(ctx, stmtHistoryInsert, downtime)
			if err != nil {
				return errors.Wrap(err, "can't insert new history entry")
			}

			return nil
		})
}
