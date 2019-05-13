package icingadb_ha

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"git.icinga.com/icingadb/icingadb-main/supervisor"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"time"
)

const (
	Notify_StartSync = iota
	Notify_StopSync
)

type HA struct {
	isActive              bool
	icinga2MTime          int64
	uid                   uuid.UUID
	super                 *supervisor.Supervisor
	notificationListeners []chan int
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

func (h *HA) icinga2HeartBeat() {
	h.icinga2MTime = time.Now().Unix()
}

func (h *HA) AreWeActive() bool {
	return h.isActive
}

func (h *HA) updateOwnInstance() error {
	_, err := h.super.Dbw.SqlExec("update icingadb_instance by id",
		fmt.Sprintf("UPDATE icingadb_instance SET heartbeat = %d WHERE id = '%s'", h.icinga2MTime, h.uid[:]))
	return err
}

func (h *HA) takeOverInstance() error {
	_, err := h.super.Dbw.SqlExec("update icingadb_instance by environment_id",
		fmt.Sprintf("UPDATE icingadb_instance SET id = '%s', heartbeat = %d WHERE environment_id = '%s'",
			h.uid[:], h.icinga2MTime, h.super.EnvId))
	return err
}

func (h *HA) insertInstance() error {
	_, err := h.super.Dbw.SqlExec("insert into icingadb_instance",
		fmt.Sprintf("INSERT INTO icingadb_instance(id, environment_id, heartbeat, responsible) VALUES ('%s', '%s', %d, 'y')",
			h.uid[:], h.super.EnvId, h.icinga2MTime))
	return err
}

func (h *HA) getInstance() (uuid.UUID, int64, error) {
	rows, err := h.super.Dbw.SqlFetchAll("select id, heartbeat from icingadb_instance where environment_id = ourEnvID",
		"SELECT id, heartbeat from icingadb_instance where environment_id = ? LIMIT 1",
		h.super.EnvId,
	)
	if err != nil {
		return uuid.UUID{}, 0, err
	}
	if len(rows) == 0 {
		return uuid.UUID{}, 0, nil
	}

	var theirUUID uuid.UUID
	copy(theirUUID[:], rows[0][0].([]byte))

	return theirUUID, rows[0][1].(int64), nil
}

func (h *HA) Run(chEnv chan *Environment) {
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

	haLogger := log.WithFields(log.Fields{
		"context":     "HA",
		"environment": hex.EncodeToString(h.super.EnvId),
		"UUID":        h.uid,
	})
	haLogger.Info("Got initial environment.")

	// We have a new UUID with every restart, no use comparing them.
	_, beat, err := h.getInstance()
	if err != nil {
		haLogger.Errorf("Failed to fetch instance: %v", err)
		h.super.ChErr <- errors.New("failed to fetch instance")
		return
	}

	if time.Now().Unix()-beat > 15 {
		haLogger.Info("Taking over.")

		// This means there was no instance row match, insert
		if beat == 0 {
			err = h.insertInstance()
		} else {
			err = h.takeOverInstance()
		}

		if err != nil {
			haLogger.Errorf("Failed to insert/update instance: %v", err)
			h.super.ChErr <- errors.New("failed to insert/update instance")
			return
		}

		h.isActive = true
		h.notifyNotificationListener(Notify_StartSync)
	} else {
		haLogger.Info("Other instance is active.")
		h.isActive = false
	}

	timerHA := time.NewTimer(time.Second * 15)
	for {
		select {
		case env := <-chEnv:
			if bytes.Compare(env.ID, h.super.EnvId) != 0 {
				haLogger.Error("Received environment is not the one we expected. Panic.")
				h.super.ChErr <- errors.New("received unexpected environment")
			}

			timerHA.Reset(time.Second * 15)
			previous := h.icinga2MTime
			h.icinga2HeartBeat()

			if h.icinga2MTime-previous < 10 {
				if h.isActive {
					err = h.updateOwnInstance()
				}
			} else {
				they, beat, err := h.getInstance()
				if err != nil {
					haLogger.Errorf("Failed to fetch instance: %v", err)
					h.super.ChErr <- errors.New("failed to fetch instance")
					return
				}
				if they == h.uid {
					haLogger.Debug("We are active.")
					if err := h.updateOwnInstance(); err != nil {
						haLogger.Errorf("Failed to update instance: %v", err)
						h.super.ChErr <- errors.New("failed to update instance")
						return
					}
				} else if h.icinga2MTime-beat > 15 {
					haLogger.Info("Taking over.")
					if err := h.takeOverInstance(); err != nil {
						haLogger.Errorf("Failed to update instance: %v", err)
						h.super.ChErr <- errors.New("failed to update instance")
					}
					h.isActive = true
					h.notifyNotificationListener(Notify_StartSync)
				} else {
					haLogger.Debug("Other instance is active.")
				}
			}
		case <-timerHA.C:
			haLogger.Info("Icinga 2 sent no heartbeat for 15 seconds, pronouncing dead.")
			h.isActive = false
			h.notifyNotificationListener(Notify_StopSync)
		}
	}
}

func (h *HA) RegisterNotificationListener() chan int {
	ch := make(chan int)
	h.notificationListeners = append(h.notificationListeners, ch)
	return ch
}

func (h *HA) notifyNotificationListener(msg int) {
	for _, c := range h.notificationListeners {
		go func(ch chan int) {
			ch <- msg
		}(c)
	}
}
