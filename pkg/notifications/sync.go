package notifications

import (
	"context"
	"crypto/sha1" // #nosec G505 -- Blocklisted import crypto/sha1
	"database/sql"
	"encoding/binary"
	"errors"
	"time"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// SyncAlertHistories periodically fetches notification histories from the API and syncs them with our DB.
//
// It runs indefinitely until the provided context is canceled, at which point it will return the context's error.
func (client *Client) SyncAlertHistories(ctx context.Context) error {
	environment, ok := v1.EnvironmentFromContext(ctx)
	if !ok {
		panic("cannot get environment from context")
	}

	selectAlertHistoryRefQ := `SELECT history_id FROM alert_history_ref WHERE history_id_uuid = ?`
	updateHistoryStmt := `UPDATE history SET alert_count = COALESCE(alert_count, 0) + 1 WHERE id = :history_id`
	insertAlertsStmt, _ := client.db.BuildInsertIgnoreStmt(AlertHistory{})

	var cursor int64
	err := retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			return client.db.GetContext(ctx, &cursor,
				client.db.Rebind(`SELECT COALESCE(MAX(triggered_at), 1) FROM alert_history WHERE environment_id = ?`), environment.Id)
		},
		retry.Retryable,
		backoff.DefaultBackoff,
		retry.Settings{})
	if err != nil {
		return err
	}

	// Each sync fetches all notification histories triggered since the cursor, and the cursor then moves back one
	// interval behind the current tick (see below). So every notification history is processed at least twice, and
	// the second time it's at least one interval old.
	//
	// This matters because a notification history can be processed before its history row exists since the history
	// row goes through the history sync pipeline, while the event is submitted separately via the runtime updates.
	// The history pipeline either writes its rows within retry.DefaultTimeout or makes Icinga DB crash fatally.
	// With one extra minute of leeway, a notification history that still has no matching history row after one
	// interval can be treated as having none (e.g., it was already deleted by the retention) and is skipped.
	const interval = retry.DefaultTimeout + time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastSync := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case tick := <-ticker.C:
			nc, ctx, cancel := client.apiClient(ctx)
			if nc == nil {
				client.logger.Debug("Cannot sync notification histories as the Notifications Client is not yet configured")
				continue
			}

			client.logger.Debugw("Syncing notification histories",
				zap.Time("last_sync", lastSync),
				zap.Time("cursor", time.UnixMilli(cursor)))

			hash := sha1.New() // #nosec G401 -- used as a non-cryptographic hash function to hash IDs.
			dbHistories := make(map[string]*AlertHistoryRef)

			// Stream all notification histories of this environment triggered since the cursor.
			historiesCh, errCh := nc.YieldNotificationHistory(ctx, map[string]string{"environment": environment.ID().String()}, cursor)

			for nh := range historiesCh {
				dbh, exists := dbHistories[nh.EventID.String()]
				if exists && dbh.HistoryID == nil {
					// We already tried to fetch the history row for this event ID, but it didn't exist yet.
					// So, skip this notification history and wait for the next sync to try again.
					continue
				} else if !exists {
					dbh = new(AlertHistoryRef)
					dbHistories[nh.EventID.String()] = dbh
					err := retry.WithBackoff(
						ctx,
						func(ctx context.Context) error {
							return client.db.GetContext(ctx, dbh, client.db.Rebind(selectAlertHistoryRefQ), nh.EventID)
						},
						retry.Retryable,
						backoff.DefaultBackoff,
						retry.Settings{Timeout: 5 * time.Second})
					if err != nil {
						if !errors.Is(err, sql.ErrNoRows) {
							client.logger.Warnw("Failed to fetch history record from DB",
								zap.Stringer("event_id", nh.EventID),
								zap.String("error", err.Error()))
						}
						// No matching alert_history_ref row (yet). Either the event was never correlatable (e.g.,
						// an older Icinga 2 or an event type without a deterministic ID), or the ref will be written
						// shortly after the event is processed. The overlapping cursor retries the latter.
						continue
					}
				}

				// Build the ID in the order documented in the schema: history_id, contact_name, contactgroup_name,
				// schedule_name, channel_name, triggered_at. Prefix each string by a zero byte to separate the
				// fields, so ("ab", "c") and ("a", "bc") don't hash the same.
				hash.Reset()
				hash.Write(dbh.HistoryID)
				for _, str := range []types.String{nh.ContactName, nh.ContactgroupName, nh.ScheduleName, nh.ChannelName} {
					_ = binary.Write(hash, binary.LittleEndian, uint8(0))
					if str.Valid {
						hash.Write([]byte(str.String))
					}
				}
				_ = binary.Write(hash, binary.LittleEndian, nh.TriggeredAt.Time().UnixMilli())

				ah := &AlertHistory{
					Id:               hash.Sum(nil),
					HistoryID:        dbh.HistoryID,
					EnvironmentId:    environment.Id,
					ContactName:      nh.ContactName,
					ContactgroupName: nh.ContactgroupName,
					ScheduleName:     nh.ScheduleName,
					ChannelName:      nh.ChannelName,
					TriggeredAt:      nh.TriggeredAt,
					EventMessage:     nh.EventMessage,
				}

				err := retry.WithBackoff(
					ctx,
					func(ctx context.Context) error {
						// In order to keep the alert count in sync with the number of alerts,
						// we need to perform the insert and update in a transaction.
						return client.db.ExecTx(ctx, nil, func(ctx context.Context, tx *sqlx.Tx) error {
							result, err := tx.NamedExecContext(ctx, insertAlertsStmt, ah)
							if err != nil {
								return err
							}
							rowsAffected, err := result.RowsAffected()
							if err != nil {
								return err
							}
							if rowsAffected == 1 {
								_, err = tx.NamedExecContext(ctx, updateHistoryStmt, dbh)
								if err != nil {
									return err
								}
							}
							return nil
						})
					},
					retry.Retryable,
					backoff.DefaultBackoff,
					retry.Settings{Timeout: 5 * time.Second})
				if err != nil {
					client.logger.Warnw("Failed to correlate notification history with history record",
						zap.Stringer("event_id", nh.EventID),
						zap.String("error", err.Error()))
				}
			}

			if err := <-errCh; err != nil {
				client.sendHeartbeat(types.MakeBool(false))
				client.logger.Warnw("Failed to fetch notification histories", zap.String("error", err.Error()))
			} else {
				// Move the cursor back one interval instead of up to the tick, so the next sync processes this window
				// again. Notification histories whose history or alert_history_ref row didn't exist yet (e.g., the
				// insert above triggered a foreign key violation) get another try once they're at least one interval
				// old. Reprocessing is safe as alert_history.id is deterministic, so the insert ignores duplicates,
				// and alert_count only grows when a row was actually inserted.
				cursor = tick.Add(-interval).UnixMilli()
			}
			cancel()

			lastSync = tick
			ticker.Reset(interval)
		}
	}
}

// deleteAlertHistoryRefs deletes all alert_history_ref rows for the given object IDs.
func (client *Client) deleteAlertHistoryRefs(ctx context.Context, isHost bool, objectIDs []types.Binary) {
	if len(objectIDs) == 0 {
		return
	}
	deleteQ := Ternary(isHost,
		`DELETE FROM alert_history_ref WHERE host_id IN (?) AND service_id IS NULL`,
		`DELETE FROM alert_history_ref WHERE service_id IN (?)`)

	deleteQ, args, err := sqlx.In(deleteQ, objectIDs)
	if err != nil {
		client.logger.Warn("Failed to prepare delete query for alert_history_ref",
			zap.Int("count", len(objectIDs)),
			zap.String("error", err.Error()))
		return
	}

	if _, err := client.db.ExecContext(ctx, client.db.Rebind(deleteQ), args...); err != nil {
		client.logger.Errorw("Failed to delete alert_history_ref rows for object IDs",
			zap.Int("count", len(objectIDs)),
			zap.String("error", err.Error()))
	}
}

// AlertHistory represents a single notification alert triggered by Icinga Notifications.
type AlertHistory struct {
	v1.IdMeta
	v1.EnvironmentMeta
	HistoryID        types.Binary    `db:"history_id"`
	ContactName      types.String    `db:"contact_name"`
	ContactgroupName types.String    `db:"contactgroup_name"`
	ScheduleName     types.String    `db:"schedule_name"`
	ChannelName      types.String    `db:"channel_name"`
	TriggeredAt      types.UnixMilli `db:"triggered_at"`
	EventMessage     types.String    `db:"event_message"`
}

// AlertHistoryRef maps a history.id to the deterministic event UUID submitted to Icinga Notifications.
//
// It's written by [Client.Submit] after an event is processed and read by [Client.SyncAlertHistories] to find
// the history row that a notification history belongs to.
type AlertHistoryRef struct {
	HistoryID     types.Binary `db:"history_id"`
	HistoryIDUUID types.UUID   `db:"history_id_uuid"`
	HostID        types.Binary `db:"host_id"`
	ServiceID     types.Binary `db:"service_id"`
}
