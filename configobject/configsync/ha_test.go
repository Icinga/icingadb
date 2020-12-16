package configsync

import (
	"github.com/Icinga/icingadb/config/testbackends"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/ha"
	"github.com/Icinga/icingadb/supervisor"
	"github.com/go-redis/redis/v7"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

func GetSuper() *supervisor.Supervisor {
	redisConn := connection.NewRDBWrapper(testbackends.RedisTestAddr, "", 64)

	return &supervisor.Supervisor{
		ChErr: make(chan error),
		Rdbw:  redisConn,
	}
}

func TestHA_NotificationListeners(t *testing.T) {
	super := GetSuper()
	chHA := make(chan ha.State)
	haInst := NewConfigSyncHA(super, chHA)

	chHost := haInst.RegisterNotificationListener("host")

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		assert.Equal(t, Notify_StartSync, <-chHost)
		wg.Done()
	}()

	haInst.notifyNotificationListener("host", Notify_StartSync)
	wg.Wait()

	chService := haInst.RegisterNotificationListener("service")
	wg.Add(1)

	go func() {
		assert.Equal(t, Notify_StartSync, <-chService)
		wg.Done()
	}()

	haInst.notifyNotificationListener("service", Notify_StartSync)
	wg.Wait()

	wg.Add(1)

	go func() {
		assert.Equal(t, Notify_StartSync, <-chService)
		assert.Equal(t, Notify_StartSync, <-chHost)
		wg.Done()
	}()

	haInst.notifyNotificationListener("*", Notify_StartSync)
	wg.Wait()
}

func TestHA_EventListener(t *testing.T) {
	super := GetSuper()
	chHA := make(chan ha.State)
	haInst := NewConfigSyncHA(super, chHA)

	haInst.Start()
	defer haInst.Stop()

	chHA <- ha.StateActive

	testbackends.RedisTestClient.Del("icinga:dump")

	chHost := haInst.RegisterNotificationListener("host")
	chService := haInst.RegisterNotificationListener("service")

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		assert.Equal(t, Notify_StartSync, <-chHost)
		assert.Equal(t, Notify_StopSync, <-chHost)
		assert.Equal(t, Notify_StartSync, <-chHost)
		assert.Equal(t, Notify_StopSync, <-chHost)
		wg.Done()
	}()

	go func() {
		assert.Equal(t, Notify_StartSync, <-chService)
		assert.Equal(t, Notify_StopSync, <-chService)
		assert.Equal(t, Notify_StartSync, <-chService)
		wg.Done()
	}()

	testbackends.RedisTestClient.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "host", "state": "done"}})
	testbackends.RedisTestClient.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "host", "state": "wip"}})
	testbackends.RedisTestClient.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "*", "state": "done"}})
	testbackends.RedisTestClient.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "*", "state": "wip"}})
	testbackends.RedisTestClient.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "service", "state": "done"}})

	wg.Wait()
}
