package ha

import (
	"crypto/sha1"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/icinga/icingadb/connection"
	"github.com/icinga/icingadb/connection/mysqld"
	"github.com/icinga/icingadb/connection/redisd"
	"github.com/icinga/icingadb/supervisor"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func createTestingHA(t *testing.T, redisAddr, mysqlHost string) *HA {
	redisConn := connection.NewRDBWrapper(redisAddr, 64)

	mysqlConn, err := connection.NewDBWrapper(fmt.Sprintf("icingadb:icingadb@%s/icingadb", mysqlHost), 50)
	if err != nil {
		assert.Fail(t, "This test needs a working Redis connection!")
	}

	super := supervisor.Supervisor{
		ChErr: make(chan error),
		Rdbw:  redisConn,
		Dbw:   mysqlConn,
	}

	ha, _ := NewHA(&super)

	hash := sha1.New()
	hash.Write([]byte("derp"))
	ha.super.EnvId = hash.Sum(nil)
	ha.uid = uuid.MustParse("551bc748-94b2-4d27-b6a4-15c52aecfe85")

	_, err = ha.super.Dbw.SqlExec(mysqlTestObserver, "TRUNCATE TABLE icingadb_instance")
	require.NoError(t, err, "This test needs a working MySQL connection!")

	ha.logger = log.WithFields(log.Fields{
		"context": "HA-Testing",
		"UUID":    ha.uid,
	})

	return ha
}

var mysqlTestObserver = connection.DbIoSeconds.WithLabelValues("mysql", "test")

func TestHA_InsertInstance(t *testing.T) {
	var redisServer redisd.Server

	client, errSrv := redisServer.Start()
	if errSrv != nil {
		t.Fatal(errSrv)
		return
	}

	defer redisServer.Stop()
	defer client.Close()

	var mysqlServer mysqld.Server

	host, errSt := mysqlServer.Start()
	if errSt != nil {
		t.Fatal(errSt)
		return
	}

	defer mysqlServer.Stop()

	if errMTD := mysqld.MkTestDb(host); errMTD != nil {
		t.Fatal(errMTD)
		return
	}

	ha := createTestingHA(t, client.Options().Addr, host)

	err := ha.insertInstance()
	require.NoError(t, err, "insertInstance should not return an error")

	rows, err := ha.super.Dbw.SqlFetchAll(mysqlObservers.selectIdHeartbeatFromIcingadbInstanceByEnvironmentId,
		"SELECT id, heartbeat from icingadb_instance where environment_id = ? LIMIT 1",
		ha.super.EnvId,
	)

	require.NoError(t, err, "There was an unexpected SQL error")
	assert.Equal(t, 1, len(rows), "There should be a row inserted")

	var theirUUID uuid.UUID
	copy(theirUUID[:], rows[0][0].([]byte))

	assert.Equal(t, ha.uid, theirUUID, "UUID must match")
}

func TestHA_checkResponsibility(t *testing.T) {
	var redisServer redisd.Server

	client, errSrv := redisServer.Start()
	if errSrv != nil {
		t.Fatal(errSrv)
		return
	}

	defer redisServer.Stop()
	defer client.Close()

	var mysqlServer mysqld.Server

	host, errSt := mysqlServer.Start()
	if errSt != nil {
		t.Fatal(errSt)
		return
	}

	defer mysqlServer.Stop()

	if errMTD := mysqld.MkTestDb(host); errMTD != nil {
		t.Fatal(errMTD)
		return
	}

	ha := createTestingHA(t, client.Options().Addr, host)
	ha.checkResponsibility()

	assert.Equal(t, true, ha.isActive, "HA should be responsible, if no other instance is active")

	_, err := ha.super.Dbw.SqlExec(mysqlTestObserver, "TRUNCATE TABLE icingadb_instance")
	require.NoError(t, err, "This test needs a working MySQL connection!")

	_, err = ha.super.Dbw.SqlExec(mysqlObservers.insertIntoIcingadbInstance,
		"INSERT INTO icingadb_instance(id, environment_id, heartbeat, responsible) VALUES (?, ?, ?, 'y')",
		ha.uid[:], ha.super.EnvId, 0)

	require.NoError(t, err, "This test needs a working MySQL connection!")

	ha.isActive = false
	ha.checkResponsibility()

	assert.Equal(t, true, ha.isActive, "HA should be responsible, if another instance was inactive for a long time")

	_, err = ha.super.Dbw.SqlExec(mysqlTestObserver, "TRUNCATE TABLE icingadb_instance")
	require.NoError(t, err, "This test needs a working MySQL connection!")

	_, err = ha.super.Dbw.SqlExec(mysqlObservers.insertIntoIcingadbInstance,
		"INSERT INTO icingadb_instance(id, environment_id, heartbeat, responsible) VALUES (?, ?, ?, 'y')",
		ha.uid[:], ha.super.EnvId, time.Now().Unix())

	ha.isActive = false
	ha.checkResponsibility()

	assert.Equal(t, false, ha.isActive, "HA should not be responsible, if another instance is active")
}

func TestHA_waitForEnvironment(t *testing.T) {
	var redisServer redisd.Server

	client, errSrv := redisServer.Start()
	if errSrv != nil {
		t.Fatal(errSrv)
		return
	}

	defer redisServer.Stop()
	defer client.Close()

	var mysqlServer mysqld.Server

	host, errSt := mysqlServer.Start()
	if errSt != nil {
		t.Fatal(errSt)
		return
	}

	defer mysqlServer.Stop()

	if errMTD := mysqld.MkTestDb(host); errMTD != nil {
		t.Fatal(errMTD)
		return
	}

	ha := createTestingHA(t, client.Options().Addr, host)

	chEnv := make(chan *Environment)

	go ha.waitForEnvironment(chEnv)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		select {
		case err := <-ha.super.ChErr:
			assert.Error(t, err, "ha.Run should return a error on empty environment")
		case <-time.NewTimer(time.Second).C:
			assert.Fail(t, "ha.Run should return a error on empty environment")
		}
		wg.Done()
	}()

	chEnv <- nil
	wg.Wait()

	wg.Add(1)

	go func() {
		ha.waitForEnvironment(chEnv)
		assert.Equal(t, []byte("my.env"), ha.super.EnvId)
		wg.Done()
	}()

	chEnv <- &Environment{ID: []byte("my.env")}
	wg.Wait()
}

func TestHA_runHA(t *testing.T) {
	var redisServer redisd.Server

	client, errSrv := redisServer.Start()
	if errSrv != nil {
		t.Fatal(errSrv)
		return
	}

	defer redisServer.Stop()
	defer client.Close()

	var mysqlServer mysqld.Server

	host, errSt := mysqlServer.Start()
	if errSt != nil {
		t.Fatal(errSt)
		return
	}

	defer mysqlServer.Stop()

	if errMTD := mysqld.MkTestDb(host); errMTD != nil {
		t.Fatal(errMTD)
		return
	}

	ha := createTestingHA(t, client.Options().Addr, host)
	ha.heartbeatTimer = time.NewTimer(10 * time.Second)

	chEnv := make(chan *Environment)

	hash1 := sha1.New()
	hash1.Write([]byte("merp"))
	ha.super.EnvId = hash1.Sum(nil)

	hash2 := sha1.New()
	hash2.Write([]byte("perp"))

	go func() {
		chEnv <- &Environment{ID: hash2.Sum(nil)}
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		timer := time.NewTimer(time.Second)
		select {
		case err := <-ha.super.ChErr:
			assert.Error(t, err, "runHA() should return an error on environment change")
		case <-timer.C:
			assert.Fail(t, "runHA() should return an error on environment change")
		}

		wg.Done()
	}()

	ha.runHA(chEnv)

	wg.Wait()
}

func TestHA_NotificationListeners(t *testing.T) {
	var redisServer redisd.Server

	client, errSrv := redisServer.Start()
	if errSrv != nil {
		t.Fatal(errSrv)
		return
	}

	defer redisServer.Stop()
	defer client.Close()

	var mysqlServer mysqld.Server

	host, errSt := mysqlServer.Start()
	if errSt != nil {
		t.Fatal(errSt)
		return
	}

	defer mysqlServer.Stop()

	if errMTD := mysqld.MkTestDb(host); errMTD != nil {
		t.Fatal(errMTD)
		return
	}

	ha := createTestingHA(t, client.Options().Addr, host)
	chHost := ha.RegisterNotificationListener("host")

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		assert.Equal(t, Notify_StartSync, <-chHost)
		wg.Done()
	}()

	ha.notifyNotificationListener("host", Notify_StartSync)
	wg.Wait()

	chService := ha.RegisterNotificationListener("service")
	wg.Add(1)

	go func() {
		assert.Equal(t, Notify_StartSync, <-chService)
		wg.Done()
	}()

	ha.notifyNotificationListener("service", Notify_StartSync)
	wg.Wait()

	wg.Add(1)

	go func() {
		assert.Equal(t, Notify_StartSync, <-chService)
		assert.Equal(t, Notify_StartSync, <-chHost)
		wg.Done()
	}()

	ha.notifyNotificationListener("*", Notify_StartSync)
	wg.Wait()
}

func TestHA_EventListener(t *testing.T) {
	var redisServer redisd.Server

	client, errSrv := redisServer.Start()
	if errSrv != nil {
		t.Fatal(errSrv)
		return
	}

	defer redisServer.Stop()

	var mysqlServer mysqld.Server

	host, errSt := mysqlServer.Start()
	if errSt != nil {
		t.Fatal(errSt)
		return
	}

	defer mysqlServer.Stop()

	if errMTD := mysqld.MkTestDb(host); errMTD != nil {
		t.Fatal(errMTD)
		return
	}

	ha := createTestingHA(t, client.Options().Addr, host)
	ha.isActive = true
	go ha.StartEventListener()

	client.Del("icinga:dump")

	chHost := ha.RegisterNotificationListener("host")
	chService := ha.RegisterNotificationListener("service")

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

	client.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "host", "state": "done"}})
	client.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "host", "state": "wip"}})
	client.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "*", "state": "done"}})
	client.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "*", "state": "wip"}})
	client.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "service", "state": "done"}})

	wg.Wait()
}
