package ha

import (
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/supervisor"
	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
	"os"
	"sync"
	"testing"
	"time"
)

func createTestingHA(t *testing.T) *HA {
	redisConn := connection.NewRDBWrapper(os.Getenv("ICINGADB_TEST_REDIS"))

	mysqlConn, err := connection.NewDBWrapper(os.Getenv("ICINGADB_TEST_MYSQL"))
	if err != nil {
		assert.Fail(t, "This test needs a working Redis connection!")
	}

	super := supervisor.Supervisor{
		ChErr:    make(chan error),
		Rdbw:     redisConn,
		Dbw:      mysqlConn,
	}

	ha, _ := NewHA(&super)
	return ha
}

func TestHA_NotificationListeners(t *testing.T) {
	ha := createTestingHA(t)
	chHost := ha.RegisterNotificationListener("host")

	go func () {
		assert.Equal(t, Notify_StartSync, <- chHost)
	}()

	ha.notifyNotificationListener("host", Notify_StartSync)

	chService := ha.RegisterNotificationListener("service")

	go func () {
		assert.Equal(t, Notify_StartSync, <- chService)
	}()

	ha.notifyNotificationListener("service", Notify_StartSync)

	go func () {
		assert.Equal(t, Notify_StartSync, <- chService)
		assert.Equal(t, Notify_StartSync, <- chHost)
	}()

	ha.notifyNotificationListener("*", Notify_StartSync)
}

func TestHA_EventListener(t *testing.T) {
	ha := createTestingHA(t)
	ha.isActive = true
	ha.StartEventListener()

	rdb := redis.NewClient(&redis.Options{
		Addr:         os.Getenv("ICINGADB_TEST_REDIS"),
		DialTimeout:  time.Minute / 2,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
	})

	rdb.Del("icinga:dump")

	chHost := ha.RegisterNotificationListener("host")
	chService := ha.RegisterNotificationListener("service")

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		assert.Equal(t, Notify_StartSync, <- chHost)
		assert.Equal(t, Notify_StopSync, <- chHost)
		assert.Equal(t, Notify_StartSync, <- chHost)
		assert.Equal(t, Notify_StopSync, <- chHost)
		wg.Done()
	}()

	go func() {
		assert.Equal(t, Notify_StartSync, <- chService)
		assert.Equal(t, Notify_StopSync, <- chService)
		assert.Equal(t, Notify_StartSync, <- chService)
		wg.Done()
	}()

	rdb.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "host", "state": "done"}})
	rdb.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "host", "state": "wip"}})
	rdb.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "*", "state": "done"}})
	rdb.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "*", "state": "wip"}})
	rdb.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "service", "state": "done"}})

	wg.Wait()
}