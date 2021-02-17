package configsync

import (
	"github.com/Icinga/icingadb/ha"
	"github.com/Icinga/icingadb/supervisor"
	"github.com/go-redis/redis/v7"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

const (
	Notify_StartSync = iota
	Notify_StopSync
)

type ConfigSyncHA struct {
	super                      *supervisor.Supervisor
	done                       chan struct{}
	chHA                       <-chan ha.State
	notificationListeners      map[string][]chan int
	notificationListenersMutex sync.Mutex
	lastEventId                string
	haIsActive                 bool
}

func NewConfigSyncHA(super *supervisor.Supervisor, chHA <-chan ha.State) *ConfigSyncHA {
	return &ConfigSyncHA{
		super:                 super,
		done:                  make(chan struct{}),
		chHA:                  chHA,
		notificationListeners: map[string][]chan int{},
		lastEventId:           "0-0",
	}
}

func (h *ConfigSyncHA) Start() {
	go h.run()
}

func (h *ConfigSyncHA) run() {
	every1s := time.NewTicker(time.Second)

loop:
	for {
		select {
		case <-h.done:
			log.Info("received done signal, shutting down")
			break loop
		case <-every1s.C:
			h.runEventListener()
		}
	}
}

func (h *ConfigSyncHA) Stop() {
	close(h.done)
}

func (h *ConfigSyncHA) runEventListener() {
	select {
	case newState := <-h.chHA:
		h.haIsActive = newState == ha.StateActive
		if !h.haIsActive {
			h.lastEventId = "0-0"
			h.notifyNotificationListener("*", Notify_StopSync)
		}
	default: // don't block if there is no change
	}

	if !h.haIsActive {
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

func (h *ConfigSyncHA) RegisterNotificationListener(listenerType string) chan int {
	ch := make(chan int, 10)
	h.notificationListenersMutex.Lock()
	h.notificationListeners[listenerType] = append(h.notificationListeners[listenerType], ch)
	h.notificationListenersMutex.Unlock()
	return ch
}

func (h *ConfigSyncHA) notifyNotificationListener(listenerType string, msg int) {
	for t, chs := range h.notificationListeners {
		if t == listenerType || listenerType == "*" {
			for _, c := range chs {
				c <- msg
			}
		}
	}
}
