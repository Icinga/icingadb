package ha

import (
	"bytes"
	"encoding/hex"
	"errors"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/icinga/icingadb/connection"
	"github.com/icinga/icingadb/supervisor"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

const (
	Notify_StartSync = iota
	Notify_StopSync
)

type HA struct {
	isActive                   bool
	lastHeartbeat              int64
	uid                        uuid.UUID
	super                      *supervisor.Supervisor
	notificationListeners      map[string][]chan int
	notificationListenersMutex sync.Mutex
	lastEventId                string
	logger                     *log.Entry
	heartbeatTimer             *time.Timer
}

func NewHA(super *supervisor.Supervisor) (*HA, error) {
	var err error
	ho := HA{
		super:                      super,
		notificationListeners:      make(map[string][]chan int),
		notificationListenersMutex: sync.Mutex{},
		lastEventId:                "0-0",
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
	selectIdHeartbeatFromIcingadbInstanceByEnvironmentId prometheus.Observer
}{
	connection.DbIoSeconds.WithLabelValues("mysql", "update icingadb_instance by id"),
	connection.DbIoSeconds.WithLabelValues("mysql", "update icingadb_instance by environment_id"),
	connection.DbIoSeconds.WithLabelValues("mysql", "insert into icingadb_instance"),
	connection.DbIoSeconds.WithLabelValues("mysql", "select id, heartbeat from icingadb_instance where environment_id = ourEnvID"),
}

func (h *HA) updateOwnInstance() error {
	_, err := h.super.Dbw.SqlExec(mysqlObservers.updateIcingadbInstanceById,
		"UPDATE icingadb_instance SET heartbeat = ? WHERE id = ?", h.lastHeartbeat, h.uid[:])
	return err
}

func (h *HA) takeOverInstance() error {
	_, err := h.super.Dbw.SqlExec(mysqlObservers.updateIcingadbInstanceByEnvironmentId,
		"UPDATE icingadb_instance SET id = ?, heartbeat = ? WHERE environment_id = ?",
		h.uid[:], h.lastHeartbeat, h.super.EnvId)
	return err
}

func (h *HA) insertInstance() error {
	_, err := h.super.Dbw.SqlExec(mysqlObservers.insertIntoIcingadbInstance,
		"INSERT INTO icingadb_instance(id, environment_id, heartbeat, responsible) VALUES (?, ?, ?, 'y')",
		h.uid[:], h.super.EnvId, h.lastHeartbeat)
	return err
}

func (h *HA) getInstance() (bool, uuid.UUID, int64, error) {
	rows, err := h.super.Dbw.SqlFetchAll(mysqlObservers.selectIdHeartbeatFromIcingadbInstanceByEnvironmentId,
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
	h.waitForEnvironment(chEnv)

	h.logger = log.WithFields(log.Fields{
		"context":     "HA",
		"environment": hex.EncodeToString(h.super.EnvId),
		"UUID":        h.uid,
	})

	h.logger.Info("Got initial environment.")

	h.checkResponsibility()

	h.heartbeatTimer = time.NewTimer(time.Second * 15)

	for {
		h.runHA(chEnv)
	}
}

func (h *HA) waitForEnvironment(chEnv chan *Environment) {
	// Wait for first heartbeat
	env := <-chEnv
	if env == nil {
		log.WithFields(log.Fields{
			"context": "HA",
		}).Error("Received empty environment.")
		h.super.ChErr <- errors.New("received empty environment")
		return
	}
	h.super.EnvId = env.ID
}

func (h *HA) checkResponsibility() {
	found, _, beat, err := h.getInstance()
	if err != nil {
		h.logger.Errorf("Failed to fetch instance: %v", err)
		h.super.ChErr <- errors.New("failed to fetch instance")
		return
	}

	if time.Now().Unix()-beat > 15 {
		h.logger.Info("Taking over.")

		// This means there was no instance row match, insert
		if !found {
			err = h.insertInstance()
		} else {
			err = h.takeOverInstance()
		}

		if err != nil {
			h.logger.Errorf("Failed to insert/update instance: %v", err)
			h.super.ChErr <- errors.New("failed to insert/update instance")
			return
		}

		h.isActive = true
	} else {
		h.logger.Info("Other instance is active.")
		h.isActive = false
		h.lastEventId = "0-0"
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
		h.lastHeartbeat = time.Now().Unix()

		if h.lastHeartbeat-previous < 10 && h.isActive {
			err := h.updateOwnInstance()

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
				if !h.isActive {
					h.logger.Info("Icinga 2 sent heartbeat. Starting sync")
					h.isActive = true
				}

				if err := h.updateOwnInstance(); err != nil {
					h.logger.Errorf("Failed to update instance: %v", err)
					h.super.ChErr <- errors.New("failed to update instance")
					return
				}
			} else if h.lastHeartbeat-beat > 15 {
				h.logger.Info("Taking over.")
				if err := h.takeOverInstance(); err != nil {
					h.logger.Errorf("Failed to update instance: %v", err)
					h.super.ChErr <- errors.New("failed to update instance")
				}
				h.isActive = true
			} else {
				h.logger.Debug("Other instance is active.")
			}
		}
	case <-h.heartbeatTimer.C:
		h.logger.Info("Icinga 2 sent no heartbeat for 15 seconds. Pausing sync")
		h.isActive = false
		h.lastEventId = "0-0"
		h.notifyNotificationListener("*", Notify_StopSync)
	}
}

func (h *HA) StartEventListener() {
	every1s := time.NewTicker(time.Second)

	for {
		<-every1s.C
		h.runEventListener()
	}
}

func (h *HA) runEventListener() {
	if !h.isActive {
		return
	}

	result := h.super.Rdbw.XRead(&redis.XReadArgs{Block: -1, Streams: []string{"icinga:dump", h.lastEventId}})
	streams, err := result.Result()
	if err != nil {
		if err.Error() != "redis: nil" {
			h.super.ChErr <- err
		}
		return
	}

	events := streams[0].Messages
	if len(events) == 0 {
		return
	}

	for _, event := range events {
		h.lastEventId = event.ID
		values := event.Values

		if values["state"] == "done" {
			h.notifyNotificationListener(values["type"].(string), Notify_StartSync)
		} else {
			h.notifyNotificationListener(values["type"].(string), Notify_StopSync)
		}
	}
}

func (h *HA) RegisterNotificationListener(listenerType string) chan int {
	ch := make(chan int, 10)
	h.notificationListenersMutex.Lock()
	h.notificationListeners[listenerType] = append(h.notificationListeners[listenerType], ch)
	h.notificationListenersMutex.Unlock()
	return ch
}

func (h *HA) notifyNotificationListener(listenerType string, msg int) {
	for t, chs := range h.notificationListeners {
		if t == listenerType || listenerType == "*" {
			for _, c := range chs {
				c <- msg
			}
		}
	}
}
