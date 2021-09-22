package icingadb

import (
	"context"
	"database/sql"
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/backoff"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	icingaredisv1 "github.com/icinga/icingadb/pkg/icingaredis/v1"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"sync"
	"time"
)

var timeout = 60 * time.Second

// HA provides high availability and indicates whether a Takeover or Handover must be made.
type HA struct {
	ctx         context.Context
	cancelCtx   context.CancelFunc
	instanceId  types.Binary
	db          *DB
	heartbeat   *icingaredis.Heartbeat
	logger      *zap.SugaredLogger
	responsible bool
	handover    chan struct{}
	takeover    chan struct{}
	done        chan struct{}
	mu          *sync.Mutex
	err         error
	errOnce     sync.Once
}

// NewHA returns a new HA and starts the controller loop.
func NewHA(ctx context.Context, db *DB, heartbeat *icingaredis.Heartbeat, logger *zap.SugaredLogger) *HA {
	ctx, cancelCtx := context.WithCancel(ctx)

	instanceId := uuid.New()

	ha := &HA{
		ctx:        ctx,
		cancelCtx:  cancelCtx,
		instanceId: instanceId[:],
		db:         db,
		heartbeat:  heartbeat,
		logger:     logger,
		handover:   make(chan struct{}),
		takeover:   make(chan struct{}),
		done:       make(chan struct{}),
		mu:         &sync.Mutex{},
	}

	go ha.controller()

	return ha
}

// Close implements the io.Closer interface.
func (h *HA) Close() error {
	// Cancel ctx.
	h.cancelCtx()
	// Wait until the controller loop ended.
	<-h.Done()
	// Remove our instance from the database.
	h.removeInstance(context.Background())
	// And return an error, if any.
	return h.Err()
}

// Done returns a channel that's closed when the HA controller loop ended.
func (h *HA) Done() <-chan struct{} {
	return h.done
}

// Err returns an error if Done has been closed and there is an error. Otherwise returns nil.
func (h *HA) Err() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.err
}

// Handover returns a channel with which handovers are signaled.
func (h *HA) Handover() chan struct{} {
	return h.handover
}

// Takeover returns a channel with which takeovers are signaled.
func (h *HA) Takeover() chan struct{} {
	return h.takeover
}

func (h *HA) abort(err error) {
	h.errOnce.Do(func() {
		h.mu.Lock()
		h.err = errors.Wrap(err, "HA aborted")
		h.mu.Unlock()

		h.cancelCtx()
	})
}

// controller loop.
func (h *HA) controller() {
	defer close(h.done)

	h.logger.Debugw("Starting HA", zap.String("instance_id", hex.EncodeToString(h.instanceId)))

	oldInstancesRemoved := false

	logTicker := time.NewTicker(time.Second * 60)
	defer logTicker.Stop()
	shouldLog := true

	for {
		select {
		case <-h.heartbeat.Beat():
			m := h.heartbeat.Message()
			now := time.Now()
			t, err := m.Time()
			if err != nil {
				h.abort(err)
			}
			tt := t.Time()
			if tt.After(now.Add(1 * time.Second)) {
				h.logger.Debugw("Received heartbeat from the future", zap.Time("time", tt))
			}
			if tt.Before(now.Add(-1 * timeout)) {
				h.logger.Errorw("Received heartbeat from the past", zap.Time("time", tt))
				h.signalHandover()
				continue
			}
			s, err := m.IcingaStatus()
			if err != nil {
				h.abort(err)
			}

			select {
			case <-logTicker.C:
				shouldLog = true
			default:
			}
			if err = h.realize(s, t, shouldLog); err != nil {
				h.abort(err)
			}
			if !oldInstancesRemoved {
				go h.removeOldInstances(s)
				oldInstancesRemoved = true
			}

			shouldLog = false
		case <-h.heartbeat.Lost():
			h.logger.Error("Lost heartbeat")
			h.signalHandover()
		case <-h.heartbeat.Done():
			if err := h.heartbeat.Err(); err != nil {
				h.abort(err)
			}
		case <-h.ctx.Done():
			return
		}
	}
}

func (h *HA) realize(s *icingaredisv1.IcingaStatus, t *types.UnixMilli, shouldLog bool) error {
	boff := backoff.NewExponentialWithJitter(time.Millisecond*256, time.Second*3)
	for attempt := 0; true; attempt++ {
		sleep := boff(uint64(attempt))
		time.Sleep(sleep)

		ctx, cancelCtx := context.WithCancel(h.ctx)
		tx, err := h.db.BeginTxx(ctx, &sql.TxOptions{
			Isolation: sql.LevelSerializable,
		})
		if err != nil {
			cancelCtx()
			return errors.Wrap(err, "can't start transaction")
		}
		query := `SELECT id, heartbeat FROM icingadb_instance WHERE environment_id = ? AND responsible = ? AND id != ? AND heartbeat > ?`
		rows, err := tx.QueryxContext(ctx, query, s.EnvironmentID(), "y", h.instanceId, utils.UnixMilli(time.Now().Add(-1*timeout)))
		if err != nil {
			cancelCtx()
			return internal.CantPerformQuery(err, query)
		}
		takeover := true
		if rows.Next() {
			instance := &v1.IcingadbInstance{}
			err := rows.StructScan(instance)
			if err != nil {
				h.logger.Errorw("Can't scan currently active instance", zap.Error(err))
			} else {
				if shouldLog {
					h.logger.Infow("Another instance is active", "instance_id", instance.Id, zap.String("environment", s.Environment), "heartbeat", instance.Heartbeat, zap.Duration("heartbeat_age", time.Since(instance.Heartbeat.Time())))
				}
				takeover = false
			}
		}
		_ = rows.Close()
		i := v1.IcingadbInstance{
			EntityWithoutChecksum: v1.EntityWithoutChecksum{
				IdMeta: v1.IdMeta{
					Id: h.instanceId,
				},
			},
			EnvironmentMeta: v1.EnvironmentMeta{
				EnvironmentId: s.EnvironmentID(),
			},
			Heartbeat:                         *t,
			Responsible:                       types.Bool{Bool: takeover || h.responsible, Valid: true},
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
		_, err = tx.NamedExecContext(ctx, stmt, i)

		if err != nil {
			cancelCtx()
			err = internal.CantPerformQuery(err, stmt)
			if !utils.IsDeadlock(err) {
				h.logger.Errorw("Can't update or insert instance", zap.Error(err))
				break
			} else {
				if attempt > 2 {
					// Log with info level after third attempt
					h.logger.Infow("Can't update or insert instance. Retrying", zap.Error(err), zap.Int("retry count", attempt))
				} else {
					h.logger.Debugw("Can't update or insert instance. Retrying", zap.Error(err), zap.Int("retry count", attempt))
				}
				continue
			}
		}

		if err := tx.Commit(); err != nil {
			cancelCtx()
			return errors.Wrap(err, "can't commit transaction")
		}
		if takeover {
			h.signalTakeover()
		}

		cancelCtx()
		break
	}

	return nil
}

func (h *HA) removeInstance(ctx context.Context) {
	h.logger.Debugw("Removing our row from icingadb_instance", zap.String("instance_id", hex.EncodeToString(h.instanceId)))
	// Intentionally not using h.ctx here as it's already cancelled.
	query := "DELETE FROM icingadb_instance WHERE id = ?"
	_, err := h.db.ExecContext(ctx, query, h.instanceId)
	if err != nil {
		h.logger.Warnw("Could not remove instance from database", zap.Error(err), zap.String("query", query))
	}
}

func (h *HA) removeOldInstances(s *icingaredisv1.IcingaStatus) {
	select {
	case <-h.ctx.Done():
		return
	case <-time.After(timeout):
		query := "DELETE FROM icingadb_instance " +
			"WHERE id != ? AND environment_id = ? AND endpoint_id = ? AND heartbeat < ?"
		heartbeat := types.UnixMilli(time.Now().Add(-timeout))
		result, err := h.db.ExecContext(h.ctx, query, h.instanceId, s.EnvironmentID(),
			s.EndpointId, heartbeat)
		if err != nil {
			h.logger.Errorw("Can't remove rows of old instances", zap.Error(err),
				zap.String("query", query),
				zap.String("id", h.instanceId.String()), zap.String("environment_id", s.EnvironmentID().String()),
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

func (h *HA) signalHandover() {
	if h.responsible {
		select {
		case h.handover <- struct{}{}:
			h.responsible = false
		case <-h.ctx.Done():
			// Noop
		}
	}
}

func (h *HA) signalTakeover() {
	if !h.responsible {
		select {
		case h.takeover <- struct{}{}:
			h.responsible = true
		case <-h.ctx.Done():
			// Noop
		}
	}
}
