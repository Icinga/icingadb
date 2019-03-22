package icingadb_ha

import (
	"bytes"
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

func newHA(super *supervisor.Supervisor) (*HA, error) {
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

func (h *HA) updateInstance() error {
	_, err := h.super.Dbw.SqlExec("update icingadb_instance by environment",
		fmt.Sprintf("UPDATE icingadb_instance SET heartbeat = %d", h.icinga2MTime))
	return err
}

func (h *HA) insertInstance() error {
	_, err := h.super.Dbw.SqlExec("insert into icingadb_instance",
		fmt.Sprintf("INSERT INTO icingadb_instance(id, environment_id, heartbeat, responsible) VALUES (%s, %s, %d, 'y')",
			h.uid, h.super.EnvId, h.icinga2MTime))
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
		log.Fatal("Environment empty?!")
	}
	h.super.EnvId = env.ID

	_, beat, err := h.getInstance()
	if err != nil {
		log.Fatal(err)
	}

	if time.Now().Unix()-beat > 15 {
		// This means there was no instance row match, insert
		if beat == 0 {
			err = h.insertInstance()
		} else {
			err = h.updateInstance()
		}
		if err != nil {
			log.Fatal(err)
		}

		h.isActive = true
		h.notifyNotificationListener(Notify_StartSync)
	} else {
		h.isActive = false
	}

	timerHA := time.NewTimer(time.Second * 15)
	for {
		select {
		case env := <-chEnv:
			if bytes.Compare(env.ID, h.super.EnvId) != 0 {
				log.Fatal("Received environment is not the one we expected. Panic.")
			}

			timerHA.Reset(time.Second * 15)
			previous := h.icinga2MTime
			h.icinga2HeartBeat()
			if h.icinga2MTime-previous < 10 {
				if h.isActive {
					err = h.updateInstance()
				}
			} else {
				they, beat, err := h.getInstance()
				if err != nil {
					log.Fatal(err)
				}
				if they == h.uid {
					if err := h.updateInstance(); err != nil {
						log.Fatal(err)
					}
				} else if h.icinga2MTime-beat > 15 {
					h.isActive = true
					h.notifyNotificationListener(Notify_StartSync)
				}
			}
		case <-timerHA.C:
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
