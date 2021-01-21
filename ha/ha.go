// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package ha

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/supervisor"
	"github.com/Icinga/icingadb/utils"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type HA struct {
	state                     State
	stateChangeListeners      []chan State
	stateChangeListenersMutex sync.Mutex
	lastHeartbeat             int64
	uid                       uuid.UUID
	super                     *supervisor.Supervisor
	logger                    *log.Entry
}

const (
	// We consider heartbeats valid for 10 seconds
	heartbeatValidMillisecs = 10 * 1000

	// We consider the heartbeat of another instance to be expired 5 seconds after its validity ended
	heartbeatTimeoutMillisecs = heartbeatValidMillisecs + 5*1000
)

func NewHA(super *supervisor.Supervisor) (*HA, error) {
	var err error
	ho := HA{
		super: super,
	}

	if ho.uid, err = uuid.NewRandom(); err != nil {
		return nil, err
	}

	return &ho, nil
}

var mysqlObservers = struct {
	updateIcingadbInstanceById                                      prometheus.Observer
	updateIcingadbInstanceByEnvironmentId                           prometheus.Observer
	insertIntoIcingadbInstance                                      prometheus.Observer
	insertIntoEnvironment                                           prometheus.Observer
	selectIdHeartbeatResponsibleFromIcingadbInstanceByEnvironmentId prometheus.Observer
	selectHeartbeatResponsibleFromIcingadbInstanceById              prometheus.Observer
	deleteIcingadbInstanceByEndpointId                              prometheus.Observer
}{
	connection.DbIoSeconds.WithLabelValues("mysql", "update icingadb_instance by id"),
	connection.DbIoSeconds.WithLabelValues("mysql", "update icingadb_instance by environment_id"),
	connection.DbIoSeconds.WithLabelValues("mysql", "insert into icingadb_instance"),
	connection.DbIoSeconds.WithLabelValues("mysql", "insert into environment"),
	connection.DbIoSeconds.WithLabelValues("mysql", "select id, heartbeat, responsible from icingadb_instance where environment_id = ourEnvID"),
	connection.DbIoSeconds.WithLabelValues("mysql", "select heartbeat, responsible from icingadb_instance by id"),
	connection.DbIoSeconds.WithLabelValues("mysql", "delete from icingadb_instance by endpoint_id"),
}

func (h *HA) setState(state State) {
	switch state {
	// valid new states (no action needed)
	case StateActive:
	case StateOtherActive:
	case StateAllInactive:
	case StateInactiveUnkown:

	// invalid arguments
	case StateInit:
		log.Fatal("Must not set HA state to StateInit")
	default:
		log.Fatalf("Trying to change to invalid HA state %d", state)
	}

	if state != h.state {
		if h.state != StateInit {
			log.Infof("Changing HA state to %s (was %s)", state.String(), h.state.String())
		} else {
			log.Infof("Changing HA state to %s", state.String())
		}

		h.state = state
		h.notifyStateChangeListeners(state)
	}
}

func (h *HA) upsertInstance(tx connection.DbTransaction, env *Environment, isActive bool) error {
	if isActive {
		// If we are active or become active, ensure that no other instance has the active flag set.
		_, err := h.super.Dbw.SqlExecTx(tx, mysqlObservers.updateIcingadbInstanceByEnvironmentId,
			"UPDATE icingadb_instance SET responsible = ? WHERE environment_id = ? AND responsible = ?",
			utils.Bool[false], h.super.EnvId, utils.Bool[true])
		if err != nil {
			return err
		}
	}

	_, err := h.super.Dbw.SqlExecTx(
		tx, mysqlObservers.insertIntoIcingadbInstance,
		"REPLACE INTO icingadb_instance(id, environment_id, endpoint_id, responsible, heartbeat,"+
			" icinga2_version, icinga2_start_time, icinga2_notifications_enabled,"+
			" icinga2_active_service_checks_enabled, icinga2_active_host_checks_enabled,"+
			" icinga2_event_handlers_enabled, icinga2_flap_detection_enabled,"+
			" icinga2_performance_data_enabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		h.uid[:],                                           // id
		h.super.EnvId,                                      // environment_id
		env.Icinga2.EndpointId,                             // endpoint_id
		utils.Bool[isActive],                               // responsible
		h.lastHeartbeat,                                    // heartbeat
		env.Icinga2.Version,                                // icinga2_version
		int64(env.Icinga2.ProgramStart*1000),               // icinga2_start_time
		utils.Bool[env.Icinga2.NotificationsEnabled],       // icinga2_notifications_enabled
		utils.Bool[env.Icinga2.ActiveServiceChecksEnabled], // icinga2_active_service_checks_enabled
		utils.Bool[env.Icinga2.ActiveHostChecksEnabled],    // icinga2_active_host_checks_enabled
		utils.Bool[env.Icinga2.EventHandlersEnabled],       // icinga2_event_handlers_enabled
		utils.Bool[env.Icinga2.FlapDetectionEnabled],       // icinga2_flap_detection_enabled
		utils.Bool[env.Icinga2.PerformanceDataEnabled],     // icinga2_performance_data_enabled
	)
	return err
}

func (h *HA) getActiveInstance(tx connection.DbTransaction) (bool, uuid.UUID, error) {
	rows, err := h.super.Dbw.SqlFetchAllTx(
		tx, mysqlObservers.selectIdHeartbeatResponsibleFromIcingadbInstanceByEnvironmentId,
		"SELECT id, heartbeat FROM icingadb_instance"+
			" WHERE environment_id = ? AND responsible = ? AND heartbeat > ?",
		h.super.EnvId, utils.Bool[true], utils.TimeToMillisecs(time.Now())-heartbeatTimeoutMillisecs,
	)
	if err != nil {
		return false, uuid.UUID{}, err
	}
	if len(rows) > 1 {
		return false, uuid.UUID{}, errors.New("there is more than one active IcingaDB instance")
	}

	if len(rows) == 0 {
		// No active instance according to database.
		return false, uuid.UUID{}, nil
	}

	idBytes := rows[0][0].([]byte)
	icinga2Heartbeat := rows[0][1].(int64)

	activeId, err := uuid.FromBytes(idBytes)
	if err != nil {
		return false, uuid.UUID{}, fmt.Errorf("invalid active UUID in database: %s", err.Error())
	}

	icinga2HeartbeatAge := utils.TimeToMillisecs(time.Now()) - icinga2Heartbeat

	if activeId == h.uid && icinga2HeartbeatAge > heartbeatValidMillisecs {
		// Our heartbeat is too old to be considered valid, no longer consider ourselves to be active.
		return false, uuid.UUID{}, nil
	} else if activeId != h.uid && icinga2HeartbeatAge > heartbeatTimeoutMillisecs {
		// Their heartbeat is old enough to be considered timed out, no longer consider them to be active.
		return false, uuid.UUID{}, nil
	}

	return true, activeId, nil
}

func (h *HA) StartHA(chEnv chan *Environment) {
	env := h.waitForEnvironment(chEnv)
	h.lastHeartbeat = utils.TimeToMillisecs(time.Now())
	err := h.setAndInsertEnvironment(env)
	if err != nil {
		h.super.ChErr <- fmt.Errorf("could not insert environment into MySQL: %s", err.Error())
	}

	h.logger = log.WithFields(log.Fields{
		"context":     "HA",
		"environment": hex.EncodeToString(h.super.EnvId),
		"UUID":        h.uid,
	})

	h.logger.Info("Got initial environment.")

	h.runHA(chEnv, env)
}

func (h *HA) waitForEnvironment(chEnv chan *Environment) *Environment {
	// Wait for first heartbeat
	env := <-chEnv
	if env == nil {
		log.WithFields(log.Fields{
			"context": "HA",
		}).Error("Received empty environment.")
		h.super.ChErr <- errors.New("received empty environment")
		return &Environment{}
	}

	return env
}

func (h *HA) setAndInsertEnvironment(env *Environment) error {
	h.super.EnvId = env.ID

	_, err := h.super.Dbw.SqlExec(
		mysqlObservers.insertIntoEnvironment,
		"REPLACE INTO environment(id, name) VALUES (?, ?)",
		env.ID, env.Name,
	)

	return err
}

// Remove rows from icingadb_instance that were created by previous startups of this instance.
// A row is considered to be created by this instance if it shares the same environment_id and
// endpoint_id. Rows with a recent heartbeat are never removed.
func (h *HA) removePreviousInstances(tx connection.DbTransaction, env *Environment) error {
	heartbeatTimeoutThreshold := utils.TimeToMillisecs(time.Now()) - heartbeatTimeoutMillisecs
	_, err := h.super.Dbw.SqlExecTx(tx, mysqlObservers.deleteIcingadbInstanceByEndpointId,
		"DELETE FROM icingadb_instance "+
			"WHERE id != ? AND environment_id = ? AND endpoint_id = ? AND heartbeat < ?",
		h.uid[:], h.super.EnvId, env.Icinga2.EndpointId, heartbeatTimeoutThreshold)
	return err
}

func (h *HA) checkResponsibility(env *Environment) {
	var newState State
	err := h.super.Dbw.SqlTransaction(true, false, false, func(tx connection.DbTransaction) error {
		err := h.removePreviousInstances(tx, env)
		if err != nil {
			return err
		}

		foundActive, activeId, err := h.getActiveInstance(tx)
		if err != nil {
			return err
		}

		lastIcinga2HeartbeatAge := utils.TimeToMillisecs(time.Now()) - h.lastHeartbeat
		lastIcinga2HeartbeatValid := lastIcinga2HeartbeatAge < heartbeatValidMillisecs

		if foundActive {
			if activeId == h.uid {
				if lastIcinga2HeartbeatValid {
					// We are active according to the DB and have a valid heartbeat, keep it that way.
					newState = StateActive
				} else {
					// We are active according to the DB but our heartbeat from Icinga 2 is no longer valid.
					// Give up active state so that another instance has a chance to take over.
					newState = StateAllInactive
				}
			} else {
				// Some other instance is active, remain passive
				newState = StateOtherActive
			}
		} else {
			// No instance is currently active. Try take over, but only
			// if we are actively receiving heartbeats from Icinga 2.
			if lastIcinga2HeartbeatValid {
				h.logger.Info("No active instance, trying to take over.")
				newState = StateActive
			} else {
				newState = StateAllInactive
			}
		}

		err = h.upsertInstance(tx, env, newState == StateActive)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		// Transaction failed, we are not sure about the current global state.
		// In any case, we ensure that we are no longer active.
		h.super.ChErr <- errors.New("HA heartbeat failed")
		h.logger.Errorf("HA heartbeat failed: %s", err.Error())
		newState = StateInactiveUnkown
	}
	h.setState(newState)
}

func (h *HA) runHA(chEnv chan *Environment, env *Environment) {
	// Force regular Icinga DB heartbeat writes to the database even if we receive no heartbeats from
	// Icinga 2. Icinga 2 will send an heartbeat every second, so use two seconds here to avoid
	// situations like forcing the update right before we receive the next heartbeat from Icinga 2.
	const updateTimerDuration = 2 * time.Second

	updateTimer := time.NewTimer(updateTimerDuration)

	for {
		select {
		case env = <-chEnv:
			if bytes.Compare(env.ID, h.super.EnvId) != 0 {
				h.logger.Error("Received environment is not the one we expected. Panic.")
				h.super.ChErr <- errors.New("received unexpected environment")
				return
			}
			h.lastHeartbeat = utils.TimeToMillisecs(time.Now())
		case <-updateTimer.C: // force update
		}

		updateTimer.Reset(updateTimerDuration)

		h.checkResponsibility(env)
	}
}

func (h *HA) RegisterStateChangeListener() <-chan State {
	// The channel has a buffer of size so that it can hold the most recent state. If it is full when we try to write,
	// we chan just drain it as the element it contains is outdated anyways.
	ch := make(chan State, 1)

	h.stateChangeListenersMutex.Lock()
	defer h.stateChangeListenersMutex.Unlock()

	h.stateChangeListeners = append(h.stateChangeListeners, ch)
	return ch
}

func (h *HA) notifyStateChangeListeners(state State) {
	h.stateChangeListenersMutex.Lock()
	defer h.stateChangeListenersMutex.Unlock()

	for _, ch := range h.stateChangeListeners {
		// drain the channel
		select {
		case <-ch:
		default:
		}

		ch <- state
	}
}
