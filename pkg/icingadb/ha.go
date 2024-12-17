package icingadb

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	icingaredisv1 "github.com/icinga/icingadb/pkg/icingaredis/v1"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
	"time"
)

// peerTimeout defines the timeout for HA heartbeats, being used to detect absent nodes.
//
// Because this timeout relies on icingaredis.Timeout, it is icingaredis.Timeout plus a short grace period.
const peerTimeout = icingaredis.Timeout + 5*time.Second

type haState struct {
	responsibleTsMilli int64
	responsible        bool
	otherResponsible   bool
}

// HA provides high availability and indicates whether a Takeover or Handover must be made.
type HA struct {
	state         atomic.Pointer[haState]
	ctx           context.Context
	cancelCtx     context.CancelFunc
	instanceId    types.Binary
	db            *database.DB
	environmentMu sync.Mutex
	environment   *v1.Environment
	heartbeat     *icingaredis.Heartbeat
	logger        *logging.Logger
	responsible   bool
	handover      chan string
	takeover      chan string
	done          chan struct{}
	errOnce       sync.Once
	errMu         sync.Mutex
	err           error
}

// NewHA returns a new HA and starts the controller loop.
func NewHA(ctx context.Context, db *database.DB, heartbeat *icingaredis.Heartbeat, logger *logging.Logger) *HA {
	ctx, cancelCtx := context.WithCancel(ctx)

	instanceId := uuid.New()

	ha := &HA{
		ctx:        ctx,
		cancelCtx:  cancelCtx,
		instanceId: instanceId[:],
		db:         db,
		heartbeat:  heartbeat,
		logger:     logger,
		handover:   make(chan string),
		takeover:   make(chan string),
		done:       make(chan struct{}),
	}

	ha.state.Store(&haState{})

	go ha.controller()

	return ha
}

// Close shuts h down.
func (h *HA) Close(ctx context.Context) error {
	// Cancel ctx.
	h.cancelCtx()
	// Wait until the controller loop ended.
	<-h.Done()
	// Remove our instance from the database.
	h.removeInstance(ctx)
	// And return an error, if any.
	return h.Err()
}

// Done returns a channel that's closed when the HA controller loop ended.
func (h *HA) Done() <-chan struct{} {
	return h.done
}

// Environment returns the current environment.
func (h *HA) Environment() *v1.Environment {
	h.environmentMu.Lock()
	defer h.environmentMu.Unlock()

	return h.environment
}

// Err returns an error if Done has been closed and there is an error. Otherwise returns nil.
func (h *HA) Err() error {
	h.errMu.Lock()
	defer h.errMu.Unlock()

	return h.err
}

// Handover returns a channel with which handovers and their reasons are signaled.
func (h *HA) Handover() chan string {
	return h.handover
}

// Takeover returns a channel with which takeovers and their reasons are signaled.
func (h *HA) Takeover() chan string {
	return h.takeover
}

// State returns the status quo.
func (h *HA) State() (responsibleTsMilli int64, responsible, otherResponsible bool) {
	state := h.state.Load()

	return state.responsibleTsMilli, state.responsible, state.otherResponsible
}

func (h *HA) abort(err error) {
	h.errOnce.Do(func() {
		h.errMu.Lock()
		h.err = errors.Wrap(err, "HA aborted")
		h.errMu.Unlock()

		h.cancelCtx()
	})
}

// controller loop.
func (h *HA) controller() {
	defer close(h.done)

	h.logger.Debugw("Starting HA", zap.String("instance_id", hex.EncodeToString(h.instanceId)))

	oldInstancesRemoved := false

	// Suppress recurring log messages in the realize method to be only logged this often.
	routineLogTicker := time.NewTicker(5 * time.Minute)
	defer routineLogTicker.Stop()
	shouldLogRoutineEvents := true

	// The retry logic in HA is twofold:
	//
	// 1) Updating or inserting the instance row based on the current heartbeat must be done within the heartbeat's
	//    expiration time. Therefore, we use a deadline ctx to retry.WithBackoff() in realize() which expires earlier
	//    than our default timeout.
	// 2) Since we do not want to exit before our default timeout expires, we have to repeat step 1 until it does.
	retryTimeout := time.NewTimer(retry.DefaultTimeout)
	defer retryTimeout.Stop()

	for {
		select {
		case <-retryTimeout.C:
			h.abort(errors.New("retry deadline exceeded"))
		case m := <-h.heartbeat.Events():
			if m != nil {
				now := time.Now()
				t, err := m.Stats().Time()
				if err != nil {
					h.abort(err)
				}
				tt := t.Time()
				if tt.After(now.Add(1 * time.Second)) {
					h.logger.Warnw("Received heartbeat from the future", zap.Time("time", tt))
				}
				if tt.Before(now.Add(-1 * peerTimeout)) {
					h.logger.Errorw("Received heartbeat from the past", zap.Time("time", tt))

					h.signalHandover("received heartbeat from the past")
					h.realizeLostHeartbeat()

					// Reset retry timeout so that the next iterations have the full amount of time available again.
					retry.ResetTimeout(retryTimeout, retry.DefaultTimeout)

					continue
				}
				s, err := m.Stats().IcingaStatus()
				if err != nil {
					h.abort(err)
				}

				envId, err := m.EnvironmentID()
				if err != nil {
					h.abort(err)
				}

				if h.environment == nil || !bytes.Equal(h.environment.Id, envId) {
					if h.environment != nil {
						h.logger.Fatalw("Environment changed unexpectedly",
							zap.String("current", h.environment.Id.String()),
							zap.String("new", envId.String()))
					}

					h.environmentMu.Lock()
					h.environment = &v1.Environment{
						EntityWithoutChecksum: v1.EntityWithoutChecksum{IdMeta: v1.IdMeta{
							Id: envId,
						}},
						Name: types.MakeString(envId.String()),
					}
					h.environmentMu.Unlock()
				}

				select {
				case <-routineLogTicker.C:
					shouldLogRoutineEvents = true
				default:
				}

				// Ensure that updating/inserting the instance row is completed by the current heartbeat's expiry time.
				realizeCtx, cancelRealizeCtx := context.WithDeadline(h.ctx, m.ExpiryTime())
				err = h.realize(realizeCtx, s, envId, shouldLogRoutineEvents)
				cancelRealizeCtx()
				if errors.Is(err, context.DeadlineExceeded) {
					h.signalHandover("instance update/insert deadline exceeded heartbeat expiry time")

					// Instance insert/update was not completed by the expiration time of the current heartbeat.
					// Pass control back to the loop to try again with the next heartbeat,
					// or exit the loop when the retry timeout has expired. Therefore,
					// retry timeout is **not** reset here so that retries continue until the timeout has expired.
					continue
				}
				if err != nil {
					h.abort(err)
				}

				if !oldInstancesRemoved {
					go h.removeOldInstances(s, envId)
					oldInstancesRemoved = true
				}

				shouldLogRoutineEvents = false
			} else {
				h.logger.Error("Lost heartbeat")
				h.signalHandover("lost heartbeat")
				h.realizeLostHeartbeat()
			}

			// Reset retry timeout so that the next iterations have the full amount of time available again.
			// Don't be surprised by the location of the code,
			// as it is obvious that the timer is also reset after an error that ends the loop anyway.
			// But this is the best place to catch all scenarios where the timeout needs to be reset.
			// And since HA needs quite a bit of refactoring anyway to e.g. return immediately after calling h.abort(),
			// it's fine to have it here for now.
			retry.ResetTimeout(retryTimeout, retry.DefaultTimeout)
		case <-h.heartbeat.Done():
			if err := h.heartbeat.Err(); err != nil {
				h.abort(err)
			}
		case <-h.ctx.Done():
			return
		}
	}
}

// realize a HA cycle triggered by a heartbeat event.
//
// The context passed is expected to have a deadline, otherwise the method will panic. This deadline is strictly
// enforced to abort the realization logic the moment the context expires.
//
// shouldLogRoutineEvents indicates if recurrent events should be logged.
//
// The internal, retryable function always fetches the last received heartbeat's timestamp instead of reusing the one
// from the calling controller loop. Doing so results in inserting a more accurate timestamp if a retry happens.
func (h *HA) realize(
	ctx context.Context,
	s *icingaredisv1.IcingaStatus,
	envId types.Binary,
	shouldLogRoutineEvents bool,
) error {
	var (
		takeover         string
		otherResponsible bool
	)

	if _, ok := ctx.Deadline(); !ok {
		panic("can't use context w/o deadline in realize()")
	}

	err := retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			takeover = ""
			otherResponsible = false
			isoLvl := sql.LevelSerializable

			if h.db.DriverName() == database.MySQL {
				// The RDBMS may actually be a Percona XtraDB Cluster which doesn't support serializable
				// transactions, but only their equivalent SELECT ... LOCK IN SHARE MODE.
				// See https://dev.mysql.com/doc/refman/8.4/en/innodb-transaction-isolation-levels.html#isolevel_serializable
				isoLvl = sql.LevelRepeatableRead
			}

			tx, errBegin := h.db.BeginTxx(ctx, &sql.TxOptions{Isolation: isoLvl})
			if errBegin != nil {
				return errors.Wrap(errBegin, "can't start transaction")
			}
			defer func() { _ = tx.Rollback() }()

			// In order to reduce the deadlocks on both sides, it is necessary to obtain an exclusive lock
			// on the selected rows. This can be achieved by utilising the SELECT ... FOR UPDATE command.
			// Nevertheless, deadlocks may still occur, when the "icingadb_instance" table is empty, i.e. when
			// there's no available row to be locked exclusively.
			//
			// Note that even without the ... FOR UPDATE lock clause, this shouldn't cause a deadlock on PostgreSQL.
			// Instead, it triggers a read/write serialization failure when attempting to commit the transaction.
			query := h.db.Rebind("SELECT id, heartbeat FROM icingadb_instance " +
				"WHERE environment_id = ? AND responsible = ? AND id <> ? FOR UPDATE")

			instance := &v1.IcingadbInstance{}
			errQuery := tx.QueryRowxContext(ctx, query, envId, "y", h.instanceId).StructScan(instance)

			switch {
			case errQuery == nil:
				fields := []any{
					zap.String("instance_id", instance.Id.String()),
					zap.String("environment", envId.String()),
					zap.Time("heartbeat", instance.Heartbeat.Time()),
					zap.Duration("heartbeat_age", time.Since(instance.Heartbeat.Time())),
				}

				if instance.Heartbeat.Time().Before(time.Now().Add(-1 * peerTimeout)) {
					takeover = "other instance's heartbeat has expired"
					h.logger.Debugw("Preparing to take over HA as other instance's heartbeat has expired", fields...)
				} else {
					otherResponsible = true
					if shouldLogRoutineEvents {
						h.logger.Infow("Another instance is active", fields...)
					}
				}

			case errors.Is(errQuery, sql.ErrNoRows):
				fields := []any{
					zap.String("instance_id", h.instanceId.String()),
					zap.String("environment", envId.String())}
				if !h.responsible {
					takeover = "no other instance is active"
					h.logger.Debugw("Preparing to take over HA as no instance is active", fields...)
				} else if h.responsible && shouldLogRoutineEvents {
					h.logger.Debugw("Continuing being the active instance", fields...)
				}

			default:
				return database.CantPerformQuery(errQuery, query)
			}

			i := v1.IcingadbInstance{
				EntityWithoutChecksum: v1.EntityWithoutChecksum{
					IdMeta: v1.IdMeta{
						Id: h.instanceId,
					},
				},
				EnvironmentMeta: v1.EnvironmentMeta{
					EnvironmentId: envId,
				},
				Heartbeat:                         types.UnixMilli(time.UnixMilli(h.heartbeat.LastMessageTime())),
				Responsible:                       types.Bool{Bool: takeover != "" || h.responsible, Valid: true},
				EndpointId:                        s.EndpointId,
				Icinga2Version:                    s.Version,
				Icinga2StartTime:                  s.ProgramStart,
				Icinga2NotificationsEnabled:       s.NotificationsEnabled,
				Icinga2ActiveServiceChecksEnabled: s.ActiveServiceChecksEnabled,
				Icinga2ActiveHostChecksEnabled:    s.ActiveHostChecksEnabled,
				Icinga2EventHandlersEnabled:       s.EventHandlersEnabled,
				Icinga2FlapDetectionEnabled:       s.FlapDetectionEnabled,
				Icinga2PerformanceDataEnabled:     s.PerformanceDataEnabled,
			}

			stmt, _ := h.db.BuildUpsertStmt(i)
			if _, err := tx.NamedExecContext(ctx, stmt, i); err != nil {
				return database.CantPerformQuery(err, stmt)
			}

			if takeover != "" {
				stmt := h.db.Rebind("UPDATE icingadb_instance SET responsible = ? WHERE environment_id = ? AND id <> ?")
				if _, err := tx.ExecContext(ctx, stmt, "n", envId, h.instanceId); err != nil {
					return database.CantPerformQuery(err, stmt)
				}

				// Insert the environment after each heartbeat takeover if it does not already exist in the database
				// as the environment may have changed, although this is likely to happen very rarely.
				stmt, _ = h.db.BuildInsertIgnoreStmt(h.environment)
				if _, err := tx.NamedExecContext(ctx, stmt, h.environment); err != nil {
					return database.CantPerformQuery(err, stmt)
				}
			}

			// In general, cancellation does not work for COMMIT and ROLLBACK. Some database drivers may support a
			// context-based abort, but only if the DBMS allows it. This was also discussed in the initial issue about
			// context support to Go's sql package: https://github.com/golang/go/issues/15123#issuecomment-245882486
			//
			// This paragraph is implementation knowledge, not covered by the API specification. Go's sql.Tx.Commit() -
			// which is not being overridden by sqlx.Tx - performs a preflight check on the context before handing over
			// to the driver's Commit() method. Drivers may behave differently. For example, the used
			// github.com/go-sql-driver/mysql package calls its internal exec() method with a COMMIT query, writing and
			// reading packets without honoring the context.
			//
			// In a nutshell, one cannot expect a Tx.Commit() call to be covered by the transaction context. For this
			// reason, the following Commit() call has been moved to its own goroutine, which communicates back via a
			// channel selected along with the context. If the context ends before Commit(), this retryable function
			// returns with a non-retryable error.
			//
			// However, while the COMMIT continues in the background, it may still succeed. In this case, the state of
			// the database does not match the state of Icinga DB, specifically the database says that this instance is
			// active while this instance thinks otherwise. Fortunately, this mismatch is not critical because when this
			// function is re-entered, the initial SELECT query would be empty for this Icinga DB node and imply the
			// presence of another active instance for the other node. Effectively, this could result in a single HA
			// cycle with no active node. Afterwards, either this instance takes over due to the false impression that
			// no other node is active, or the other instances does so as the inserted heartbeat has already expired.
			// Not great, not terrible.
			commitErrCh := make(chan error, 1)
			go func() { commitErrCh <- tx.Commit() }()

			select {
			case err := <-commitErrCh:
				if err != nil {
					return errors.Wrap(err, "can't commit transaction")
				}
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		},
		retry.Retryable,
		backoff.NewExponentialWithJitter(256*time.Millisecond, 3*time.Second),
		retry.Settings{
			// Intentionally no timeout is set, as we use a context with a deadline.
			OnRetryableError: func(_ time.Duration, attempt uint64, err, lastErr error) {
				if lastErr == nil || err.Error() != lastErr.Error() {
					log := h.logger.Debugw
					if attempt > 3 {
						log = h.logger.Infow
					}

					log("Can't update or insert instance. Retrying", zap.Error(err))
				}
			},
			OnSuccess: func(elapsed time.Duration, attempt uint64, lastErr error) {
				if attempt > 1 {
					log := h.logger.Debugw

					if attempt > 4 {
						// We log errors with severity info starting from the fourth attempt, (see above)
						// so we need to log success with severity info from the fifth attempt.
						log = h.logger.Infow
					}

					log("Instance updated/inserted successfully after error",
						zap.Duration("after", elapsed),
						zap.Uint64("attempts", attempt),
						zap.NamedError("recovered_error", lastErr))
				}
			},
		},
	)
	if err != nil {
		return err
	}

	if takeover != "" {
		h.signalTakeover(takeover)
	} else if otherResponsible {
		if state := h.state.Load(); state.responsible {
			h.logger.Error("Other instance is responsible while this node itself is responsible, dropping responsibility")
			h.signalHandover("other instance is responsible as well")
			// h.signalHandover will update h.state
		}
		if state := h.state.Load(); !state.otherResponsible {
			// Dereference pointer to create a copy of the value it points to.
			// Ensures that any modifications do not directly affect the original data unless explicitly stored back.
			newState := *state
			newState.otherResponsible = true
			h.state.Store(&newState)
		}
	}

	return nil
}

// realizeLostHeartbeat updates "responsible = n" for this HA into the database.
func (h *HA) realizeLostHeartbeat() {
	stmt := h.db.Rebind("UPDATE icingadb_instance SET responsible = ? WHERE id = ?")
	if _, err := h.db.ExecContext(h.ctx, stmt, "n", h.instanceId); err != nil && !utils.IsContextCanceled(err) {
		h.logger.Warnw("Can't update instance", zap.Error(database.CantPerformQuery(err, stmt)))
	}
}

func (h *HA) removeInstance(ctx context.Context) {
	h.logger.Debugw("Removing our row from icingadb_instance", zap.String("instance_id", hex.EncodeToString(h.instanceId)))
	// Intentionally not using h.ctx here as it's already cancelled.
	query := h.db.Rebind("DELETE FROM icingadb_instance WHERE id = ?")
	_, err := h.db.ExecContext(ctx, query, h.instanceId)
	if err != nil {
		h.logger.Warnw("Could not remove instance from database", zap.Error(err), zap.String("query", query))
	}
}

func (h *HA) removeOldInstances(s *icingaredisv1.IcingaStatus, envId types.Binary) {
	select {
	case <-h.ctx.Done():
		return
	case <-time.After(peerTimeout):
		query := h.db.Rebind("DELETE FROM icingadb_instance " +
			"WHERE id <> ? AND environment_id = ? AND endpoint_id = ? AND heartbeat < ?")
		heartbeat := types.UnixMilli(time.Now().Add(-1 * peerTimeout))
		result, err := h.db.ExecContext(h.ctx, query, h.instanceId, envId,
			s.EndpointId, heartbeat)
		if err != nil {
			h.logger.Errorw("Can't remove rows of old instances", zap.Error(err),
				zap.String("query", query),
				zap.String("id", h.instanceId.String()), zap.String("environment_id", envId.String()),
				zap.String("endpoint_id", s.EndpointId.String()), zap.Time("heartbeat", heartbeat.Time()))
			return
		}
		affected, err := result.RowsAffected()
		if err != nil {
			h.logger.Errorw("Can't get number of removed old instances", zap.Error(err))
			return
		}
		h.logger.Debugf("Removed %d old instances", affected)
	}
}

// signalHandover gives up HA.responsible and notifies the HA.Handover chan.
func (h *HA) signalHandover(reason string) {
	if h.responsible {
		h.state.Store(&haState{
			responsibleTsMilli: time.Now().UnixMilli(),
			responsible:        false,
			otherResponsible:   false,
		})

		select {
		case h.handover <- reason:
			h.responsible = false
		case <-h.ctx.Done():
			// Noop
		}
	}
}

// signalTakeover claims HA.responsible and notifies the HA.Takeover chan.
func (h *HA) signalTakeover(reason string) {
	if !h.responsible {
		h.state.Store(&haState{
			responsibleTsMilli: time.Now().UnixMilli(),
			responsible:        true,
			otherResponsible:   false,
		})

		select {
		case h.takeover <- reason:
			h.responsible = true
		case <-h.ctx.Done():
			// Noop
		}
	}
}
