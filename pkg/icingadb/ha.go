package icingadb

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/backoff"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/driver"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	icingaredisv1 "github.com/icinga/icingadb/pkg/icingaredis/v1"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/retry"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"sync"
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
	state         com.Atomic[haState]
	ctx           context.Context
	cancelCtx     context.CancelFunc
	instanceId    types.Binary
	db            *DB
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
func NewHA(ctx context.Context, db *DB, heartbeat *icingaredis.Heartbeat, logger *logging.Logger) *HA {
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
	state, _ := h.state.Load()
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

	for {
		select {
		case m := <-h.heartbeat.Events():
			if m != nil {
				now := time.Now()
				t, err := m.Stats().Time()
				if err != nil {
					h.abort(err)
				}
				tt := t.Time()
				if tt.After(now.Add(1 * time.Second)) {
					h.logger.Debugw("Received heartbeat from the future", zap.Time("time", tt))
				}
				if tt.Before(now.Add(-1 * peerTimeout)) {
					h.logger.Errorw("Received heartbeat from the past", zap.Time("time", tt))
					h.signalHandover("received heartbeat from the past")
					h.realizeLostHeartbeat()
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

				var realizeCtx context.Context
				var cancelRealizeCtx context.CancelFunc
				if h.responsible {
					realizeCtx, cancelRealizeCtx = context.WithDeadline(h.ctx, m.ExpiryTime())
				} else {
					realizeCtx, cancelRealizeCtx = context.WithCancel(h.ctx)
				}
				err = h.realize(realizeCtx, s, t, envId, shouldLogRoutineEvents)
				cancelRealizeCtx()
				if errors.Is(err, context.DeadlineExceeded) {
					h.signalHandover("context deadline exceeded")
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
// shouldLogRoutineEvents indicates if recurrent events should be logged.
func (h *HA) realize(
	ctx context.Context,
	s *icingaredisv1.IcingaStatus,
	t *types.UnixMilli,
	envId types.Binary,
	shouldLogRoutineEvents bool,
) error {
	var (
		takeover         string
		otherResponsible bool
	)

	err := retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			takeover = ""
			otherResponsible = false
			isoLvl := sql.LevelSerializable
			selectLock := ""

			if h.db.DriverName() == driver.MySQL {
				// The RDBMS may actually be a Percona XtraDB Cluster which doesn't
				// support serializable transactions, but only their following equivalent:
				isoLvl = sql.LevelRepeatableRead
				selectLock = " LOCK IN SHARE MODE"
			}

			tx, errBegin := h.db.BeginTxx(ctx, &sql.TxOptions{Isolation: isoLvl})
			if errBegin != nil {
				return errors.Wrap(errBegin, "can't start transaction")
			}

			query := h.db.Rebind("SELECT id, heartbeat FROM icingadb_instance "+
				"WHERE environment_id = ? AND responsible = ? AND id <> ?") + selectLock

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
				return internal.CantPerformQuery(errQuery, query)
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
				Heartbeat:                         *t,
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
				return internal.CantPerformQuery(err, stmt)
			}

			if takeover != "" {
				stmt := h.db.Rebind("UPDATE icingadb_instance SET responsible = ? WHERE environment_id = ? AND id <> ?")
				_, err := tx.ExecContext(ctx, stmt, "n", envId, h.instanceId)

				if err != nil {
					return internal.CantPerformQuery(err, stmt)
				}
			}

			if err := tx.Commit(); err != nil {
				return errors.Wrap(err, "can't commit transaction")
			}

			return nil
		},
		retry.Retryable,
		backoff.NewExponentialWithJitter(time.Millisecond*256, time.Second*3),
		retry.Settings{
			OnError: func(_ time.Duration, attempt uint64, err, lastErr error) {
				if lastErr == nil || err.Error() != lastErr.Error() {
					log := h.logger.Debugw
					if attempt > 2 {
						log = h.logger.Infow
					}

					log("Can't update or insert instance. Retrying", zap.Error(err), zap.Uint64("retry count", attempt))
				}
			},
		},
	)
	if err != nil {
		return err
	}

	if takeover != "" {
		// Insert the environment after each heartbeat takeover if it does not already exist in the database
		// as the environment may have changed, although this is likely to happen very rarely.
		if err := h.insertEnvironment(); err != nil {
			return errors.Wrap(err, "can't insert environment")
		}

		h.signalTakeover(takeover)
	} else if otherResponsible {
		if state, _ := h.state.Load(); !state.otherResponsible {
			state.otherResponsible = true
			h.state.Store(state)
		}
	}

	return nil
}

// realizeLostHeartbeat updates "responsible = n" for this HA into the database.
func (h *HA) realizeLostHeartbeat() {
	stmt := h.db.Rebind("UPDATE icingadb_instance SET responsible = ? WHERE id = ?")
	if _, err := h.db.ExecContext(h.ctx, stmt, "n", h.instanceId); err != nil && !utils.IsContextCanceled(err) {
		h.logger.Warnw("Can't update instance", zap.Error(internal.CantPerformQuery(err, stmt)))
	}
}

// insertEnvironment inserts the environment from the specified state into the database if it does not already exist.
func (h *HA) insertEnvironment() error {
	// Instead of checking whether the environment already exists, use an INSERT statement that does nothing if it does.
	stmt, _ := h.db.BuildInsertIgnoreStmt(h.environment)

	if _, err := h.db.NamedExecContext(h.ctx, stmt, h.environment); err != nil {
		return internal.CantPerformQuery(err, stmt)
	}

	return nil
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
		h.state.Store(haState{
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
		h.state.Store(haState{
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
