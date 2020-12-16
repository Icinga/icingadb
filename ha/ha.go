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
	heartbeatTimer            *time.Timer
}

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
	updateIcingadbInstanceById                           prometheus.Observer
	updateIcingadbInstanceByEnvironmentId                prometheus.Observer
	insertIntoIcingadbInstance                           prometheus.Observer
	insertIntoEnvironment                                prometheus.Observer
	selectIdHeartbeatFromIcingadbInstanceByEnvironmentId prometheus.Observer
}{
	connection.DbIoSeconds.WithLabelValues("mysql", "update icingadb_instance by id"),
	connection.DbIoSeconds.WithLabelValues("mysql", "update icingadb_instance by environment_id"),
	connection.DbIoSeconds.WithLabelValues("mysql", "insert into icingadb_instance"),
	connection.DbIoSeconds.WithLabelValues("mysql", "insert into environment"),
	connection.DbIoSeconds.WithLabelValues("mysql", "select id, heartbeat from icingadb_instance where environment_id = ourEnvID"),
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

func (h *HA) updateOwnInstance(env *Environment) error {
	_, err := h.super.Dbw.SqlExec(
		mysqlObservers.insertIntoIcingadbInstance,
		"REPLACE INTO icingadb_instance(id, environment_id, endpoint_id, heartbeat, responsible,"+
			" icinga2_version, icinga2_start_time, icinga2_notifications_enabled,"+
			" icinga2_active_service_checks_enabled, icinga2_active_host_checks_enabled,"+
			" icinga2_event_handlers_enabled, icinga2_flap_detection_enabled,"+
			" icinga2_performance_data_enabled) VALUES (?, ?, ?, ?, 'y', ?, ?, ?, ?, ?, ?, ?, ?)",
		h.uid[:],
		h.super.EnvId,
		env.Icinga2.EndpointId,
		h.lastHeartbeat,
		env.Icinga2.Version,
		int64(env.Icinga2.ProgramStart*1000),
		utils.Bool[env.Icinga2.NotificationsEnabled],
		utils.Bool[env.Icinga2.ActiveServiceChecksEnabled],
		utils.Bool[env.Icinga2.ActiveHostChecksEnabled],
		utils.Bool[env.Icinga2.EventHandlersEnabled],
		utils.Bool[env.Icinga2.FlapDetectionEnabled],
		utils.Bool[env.Icinga2.PerformanceDataEnabled],
	)
	return err
}

func (h *HA) takeOverInstance(env *Environment) error {
	_, err := h.super.Dbw.SqlExec(
		mysqlObservers.updateIcingadbInstanceByEnvironmentId,
		"UPDATE icingadb_instance SET id = ?, endpoint_id = ?, heartbeat = ?,"+
			" icinga2_version = ?, icinga2_start_time = ?, icinga2_notifications_enabled = ?,"+
			" icinga2_active_service_checks_enabled = ?, icinga2_active_host_checks_enabled = ?,"+
			" icinga2_event_handlers_enabled = ?, icinga2_flap_detection_enabled = ?,"+
			" icinga2_performance_data_enabled = ? WHERE environment_id = ?",
		h.uid[:],
		env.Icinga2.EndpointId,
		h.lastHeartbeat,
		env.Icinga2.Version,
		int64(env.Icinga2.ProgramStart*1000),
		utils.Bool[env.Icinga2.NotificationsEnabled],
		utils.Bool[env.Icinga2.ActiveServiceChecksEnabled],
		utils.Bool[env.Icinga2.ActiveHostChecksEnabled],
		utils.Bool[env.Icinga2.EventHandlersEnabled],
		utils.Bool[env.Icinga2.FlapDetectionEnabled],
		utils.Bool[env.Icinga2.PerformanceDataEnabled],
		h.super.EnvId,
	)
	return err
}

func (h *HA) insertInstance(env *Environment) error {
	_, err := h.super.Dbw.SqlExec(
		mysqlObservers.insertIntoIcingadbInstance,
		"INSERT INTO icingadb_instance(id, environment_id, endpoint_id, heartbeat, responsible,"+
			" icinga2_version, icinga2_start_time, icinga2_notifications_enabled,"+
			" icinga2_active_service_checks_enabled, icinga2_active_host_checks_enabled,"+
			" icinga2_event_handlers_enabled, icinga2_flap_detection_enabled,"+
			" icinga2_performance_data_enabled) VALUES (?, ?, ?, ?, 'y', ?, ?, ?, ?, ?, ?, ?, ?)",
		h.uid[:],
		h.super.EnvId,
		env.Icinga2.EndpointId,
		h.lastHeartbeat,
		env.Icinga2.Version,
		int64(env.Icinga2.ProgramStart*1000),
		utils.Bool[env.Icinga2.NotificationsEnabled],
		utils.Bool[env.Icinga2.ActiveServiceChecksEnabled],
		utils.Bool[env.Icinga2.ActiveHostChecksEnabled],
		utils.Bool[env.Icinga2.EventHandlersEnabled],
		utils.Bool[env.Icinga2.FlapDetectionEnabled],
		utils.Bool[env.Icinga2.PerformanceDataEnabled],
	)
	return err
}

func (h *HA) getInstance() (bool, uuid.UUID, int64, error) {
	rows, err := h.super.Dbw.SqlFetchAll(
		mysqlObservers.selectIdHeartbeatFromIcingadbInstanceByEnvironmentId,
		"SELECT id, heartbeat from icingadb_instance where environment_id = ? LIMIT 1",
		h.super.EnvId,
	)

	if err != nil {
		return false, uuid.UUID{}, 0, err
	}
	if len(rows) == 0 {
		return false, uuid.UUID{}, 0, nil
	}

	var theirUUID uuid.UUID
	copy(theirUUID[:], rows[0][0].([]byte))

	return true, theirUUID, rows[0][1].(int64), nil
}

func (h *HA) StartHA(chEnv chan *Environment) {
	env := h.waitForEnvironment(chEnv)
	err := h.setAndInsertEnvironment(env)
	if err != nil {
		h.super.ChErr <- fmt.Errorf("Could not insert environment into MySQL: %s", err.Error())
	}

	h.logger = log.WithFields(log.Fields{
		"context":     "HA",
		"environment": hex.EncodeToString(h.super.EnvId),
		"UUID":        h.uid,
	})

	h.logger.Info("Got initial environment.")

	h.checkResponsibility(env)

	h.heartbeatTimer = time.NewTimer(time.Second * 15)

	for {
		h.runHA(chEnv)
	}
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

func (h *HA) checkResponsibility(env *Environment) {
	found, _, beat, err := h.getInstance()
	if err != nil {
		h.logger.Errorf("Failed to fetch instance: %v", err)
		h.super.ChErr <- errors.New("failed to fetch instance")
		return
	}

	if utils.TimeToMillisecs(time.Now())-beat > 15*1000 {
		h.logger.Info("Taking over.")

		// This means there was no instance row match, insert
		if !found {
			err = h.insertInstance(env)
		} else {
			err = h.takeOverInstance(env)
		}

		if err != nil {
			h.logger.Errorf("Failed to insert/update instance: %v", err)
			h.super.ChErr <- errors.New("failed to insert/update instance")
			return
		}

		h.setState(StateActive)
	} else {
		h.logger.Info("Other instance is active.")
		h.setState(StateOtherActive)
	}
}

func (h *HA) runHA(chEnv chan *Environment) {
	select {
	case env := <-chEnv:
		if bytes.Compare(env.ID, h.super.EnvId) != 0 {
			h.logger.Error("Received environment is not the one we expected. Panic.")
			h.super.ChErr <- errors.New("received unexpected environment")
			return
		}

		h.heartbeatTimer.Reset(time.Second * 15)
		previous := h.lastHeartbeat
		h.lastHeartbeat = utils.TimeToMillisecs(time.Now())

		if h.lastHeartbeat-previous < 10*1000 && h.state == StateActive {
			err := h.updateOwnInstance(env)

			if err != nil {
				h.logger.Errorf("Failed to update instance: %v", err)
				h.super.ChErr <- errors.New("failed to update instance")
				return
			}
		} else {
			_, they, beat, err := h.getInstance()
			if err != nil {
				h.logger.Errorf("Failed to fetch instance: %v", err)
				h.super.ChErr <- errors.New("failed to fetch instance")
				return
			}
			if they == h.uid {
				h.logger.Debug("We are active.")
				if h.state != StateActive {
					h.logger.Info("Icinga 2 sent heartbeat. Starting sync")
					h.setState(StateActive)
				}

				if err := h.updateOwnInstance(env); err != nil {
					h.logger.Errorf("Failed to update instance: %v", err)
					h.super.ChErr <- errors.New("failed to update instance")
					return
				}
			} else if h.lastHeartbeat-beat > 15*1000 {
				h.logger.Info("Taking over.")
				if err := h.takeOverInstance(env); err != nil {
					h.logger.Errorf("Failed to update instance: %v", err)
					h.super.ChErr <- errors.New("failed to update instance")
				}
				h.setState(StateActive)
			} else {
				h.logger.Debug("Other instance is active.")
			}
		}
	case <-h.heartbeatTimer.C:
		h.logger.Info("Icinga 2 sent no heartbeat for 15 seconds. Pausing sync")
		h.setState(StateAllInactive)
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
