package icingadb

import (
	"context"
	"database/sql"
	"encoding/hex"
	"github.com/google/uuid"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	icingaredisv1 "github.com/icinga/icingadb/pkg/icingaredis/v1"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/icinga/icingadb/pkg/utils"
	"go.uber.org/zap"
	"sync"
	"time"
)

var timeout = 60 * time.Second

type HA struct {
	ctx         context.Context
	cancel      context.CancelFunc
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

func NewHA(ctx context.Context, db *DB, heartbeat *icingaredis.Heartbeat, logger *zap.SugaredLogger) *HA {
	ctx, cancel := context.WithCancel(ctx)

	instanceId := uuid.New()

	ha := &HA{
		ctx:        ctx,
		cancel:     cancel,
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
	h.cancel()
	// Wait until the controller loop ended.
	<-h.Done()
	// Remove our instance from the database.
	h.removeInstance()
	// And return an error, if any.
	return h.Err()
}

func (h *HA) Done() <-chan struct{} {
	return h.done
}

func (h *HA) Err() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.err
}

func (h *HA) Handover() chan struct{} {
	return h.handover
}

func (h *HA) Takeover() chan struct{} {
	return h.takeover
}

func (h *HA) abort(err error) {
	h.errOnce.Do(func() {
		h.mu.Lock()
		h.err = err
		h.mu.Unlock()

		h.cancel()
	})
}

// controller loop.
func (h *HA) controller() {
	defer close(h.done)

	h.logger.Debugw("Starting HA", zap.String("instance_id", hex.EncodeToString(h.instanceId)))

	oldInstancesRemoved := false

	for {
		select {
		case m, ok := <-h.heartbeat.Beat():
			if !ok {
				// Beat channel closed.
				return
			}
			now := time.Now()
			t, err := m.Time()
			if err != nil {
				h.abort(err)
			}
			tt := t.Time()
			if tt.After(now.Add(1 * time.Second)) {
				h.logger.Debugw("Received heartbeat from future", "time", t)
			}
			if tt.Before(now.Add(-1 * timeout)) {
				h.logger.Errorw("Received heartbeat from the past %s", "time", t)
				h.signalHandover()
				continue
			}
			s, err := m.IcingaStatus()
			if err != nil {
				h.abort(err)
			}
			if err = h.realize(s, t); err != nil {
				h.abort(err)
			}
			if !oldInstancesRemoved {
				go h.removeOldInstances(s)
				oldInstancesRemoved = true
			}
		case <-h.heartbeat.Lost():
			h.logger.Error("Lost heartbeat")
			h.signalHandover()
		case <-h.ctx.Done():
			return
		}
	}
}

func (h *HA) realize(s *icingaredisv1.IcingaStatus, t *types.UnixMilli) error {
	// boff := backoff.NewExponentialWithJitter(time.Millisecond*1, time.Second*1)
	for attempt := 0; true; attempt++ {
		// sleep := boff(uint64(attempt))
		// h.logger.Debugf("Sleeping for %s..", sleep)
		// time.Sleep(sleep)
		ctx, cancel := context.WithCancel(h.ctx)
		tx, err := h.db.BeginTxx(ctx, &sql.TxOptions{
			Isolation: sql.LevelSerializable,
		})
		if err != nil {
			return err
		}
		rows, err := tx.QueryxContext(ctx, `SELECT 1 FROM icingadb_instance WHERE environment_id = ? AND responsible = ? AND heartbeat > ?`, s.EnvironmentID(), "y", utils.UnixMilli(time.Now().Add(-1*timeout)))
		if err != nil {
			return err
		}
		takeover := true
		for rows.Next() {
			takeover = false
			break
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
			cancel()
			if !utils.IsDeadlock(err) {
				h.logger.Errorw("Can't Update or insert instance.", zap.Error(err))
				break
			} else {
				h.logger.Infow("Can't Update or insert instance. Retrying..", zap.Error(err))
				continue
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		if takeover {
			h.signalTakeover()
		}
		break
	}

	return nil
}

func (h *HA) removeInstance() {
	h.logger.Debugw("Removing our row from icingadb_instance", zap.String("instance_id", hex.EncodeToString(h.instanceId)))
	// Intentionally not using a context here as this is a cleanup task and h.ctx is already cancelled.
	_, err := h.db.Exec("DELETE FROM icingadb_instance WHERE id = ?", h.instanceId)
	if err != nil {
		h.logger.Warnw("Could not remove instance from database", zap.Error(err))
	}
}

func (h *HA) removeOldInstances(s *icingaredisv1.IcingaStatus) {
	select {
	case <-h.ctx.Done():
		return
	case <-time.After(timeout):
		result, err := h.db.ExecContext(h.ctx, "DELETE FROM icingadb_instance "+
			"WHERE id != ? AND environment_id = ? AND endpoint_id = ? AND heartbeat < ?",
			h.instanceId, s.EnvironmentID(), s.EndpointId, types.UnixMilli(time.Now().Add(-timeout)))
		if err != nil {
			h.logger.Errorw("Can't remove rows of old instances", zap.Error(err))
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
		h.logger.Warn("Handing over..")
		h.responsible = false
		h.handover <- struct{}{}
	}
}

func (h *HA) signalTakeover() {
	if !h.responsible {
		h.logger.Info("Taking over..")
		h.responsible = true
		h.takeover <- struct{}{}
	}
}
