// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package ha

import (
	"crypto/sha1"
	"github.com/Icinga/icingadb/config/testbackends"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/supervisor"
	"github.com/Icinga/icingadb/utils"
	"github.com/go-redis/redis/v7"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func createTestingHA(t *testing.T, redisAddr string) *HA {
	driver, info, errDI := testbackends.GetDbInfo()
	if errDI != nil {
		t.Fatal(errDI)
	}

	redisConn := connection.NewRDBWrapper(redisAddr, 64)

	dbConn, err := connection.NewDBWrapper(driver, info)
	if err != nil {
		assert.Fail(t, "This test needs a working Redis connection!")
	}

	super := supervisor.Supervisor{
		ChErr: make(chan error),
		Rdbw:  redisConn,
		Dbw:   dbConn,
	}

	ha, _ := NewHA(&super)

	hash := sha1.New()
	hash.Write([]byte("derp"))
	ha.super.EnvId = hash.Sum(nil)
	ha.uid = uuid.MustParse("551bc748-94b2-4d27-b6a4-15c52aecfe85")

	_, err = ha.super.Dbw.SqlExec(dbTestObserver, "TRUNCATE TABLE icingadb_instance")
	require.NoError(t, err, "This test needs a working database connection!")

	ha.logger = log.WithFields(log.Fields{
		"context": "HA-Testing",
		"UUID":    ha.uid,
	})

	return ha
}

var dbTestObserver = connection.DbIoSeconds.WithLabelValues("rdbms", "test")

func TestHA_InsertInstance(t *testing.T) {
	type row struct {
		Id        uuid.UUID
		Heartbeat uint64
	}

	ha := createTestingHA(t, testbackends.RedisTestAddr)

	err := ha.insertInstance(&Environment{})
	require.NoError(t, err, "insertInstance should not return an error")

	rawRows, err := ha.super.Dbw.SqlFetchAll(
		dbObservers.selectIdHeartbeatFromIcingadbInstanceByEnvironmentId, row{},
		"SELECT id, heartbeat from icingadb_instance where environment_id = "+
			connection.Placeholders(ha.super.Dbw.Db, 0, 1)+" LIMIT 1",
		ha.super.EnvId,
	)

	require.NoError(t, err, "There was an unexpected SQL error")

	rows := rawRows.([]row)

	assert.Equal(t, 1, len(rows), "There should be a row inserted")
	assert.Equal(t, ha.uid, rows[0].Id, "UUID must match")
}

func TestHA_checkResponsibility(t *testing.T) {
	ha := createTestingHA(t, testbackends.RedisTestAddr)
	ha.checkResponsibility(&Environment{})

	assert.Equal(t, true, ha.isActive, "HA should be responsible, if no other instance is active")

	_, err := ha.super.Dbw.SqlExec(dbTestObserver, "TRUNCATE TABLE icingadb_instance")
	require.NoError(t, err, "This test needs a working database connection!")

	_, err = ha.super.Dbw.SqlExec(
		dbObservers.insertIntoIcingadbInstance,
		connection.Insert(
			ha.super.Dbw.Db, "icingadb_instance",
			"id", "environment_id", "heartbeat", "responsible", "icinga2_version", "icinga2_start_time",
			"icinga2_notifications_enabled", "icinga2_active_service_checks_enabled",
			"icinga2_active_host_checks_enabled", "icinga2_event_handlers_enabled", "icinga2_flap_detection_enabled",
			"icinga2_performance_data_enabled",
		),
		ha.uid[:], ha.super.EnvId, 0, "y", "", 0, "y", "y", "y", "y", "y", "y",
	)

	require.NoError(t, err, "This test needs a working database connection!")

	ha.isActive = false
	ha.checkResponsibility(&Environment{})

	assert.Equal(t, true, ha.isActive, "HA should be responsible, if another instance was inactive for a long time")

	_, err = ha.super.Dbw.SqlExec(dbTestObserver, "TRUNCATE TABLE icingadb_instance")
	require.NoError(t, err, "This test needs a working database connection!")

	_, err = ha.super.Dbw.SqlExec(
		dbObservers.insertIntoIcingadbInstance,
		connection.Insert(
			ha.super.Dbw.Db, "icingadb_instance",
			"id", "environment_id", "heartbeat", "responsible", "icinga2_version", "icinga2_start_time",
			"icinga2_notifications_enabled", "icinga2_active_service_checks_enabled",
			"icinga2_active_host_checks_enabled", "icinga2_event_handlers_enabled", "icinga2_flap_detection_enabled",
			"icinga2_performance_data_enabled",
		),
		ha.uid[:], ha.super.EnvId, utils.TimeToMillisecs(time.Now()), "y", "", 0, "y", "y", "y", "y", "y", "y",
	)

	ha.isActive = false
	ha.checkResponsibility(&Environment{})

	assert.Equal(t, false, ha.isActive, "HA should not be responsible, if another instance is active")
}

func TestHA_waitForEnvironment(t *testing.T) {
	ha := createTestingHA(t, testbackends.RedisTestAddr)

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
		env := ha.waitForEnvironment(chEnv)
		assert.Equal(t, []byte("my.env"), env.ID)
		wg.Done()
	}()

	chEnv <- &Environment{ID: []byte("my.env")}
	wg.Wait()
}

func TestHA_setAndInsertEnvironment(t *testing.T) {
	type row struct {
		Name string
	}

	ha := createTestingHA(t, testbackends.RedisTestAddr)

	env := Environment{
		ID: utils.EncodeChecksum(utils.Checksum("herp")),
		Name: "herp",
	}

	err := ha.setAndInsertEnvironment(&env)
	require.NoError(t, err, "setAndInsertEnvironment should not return an error")

	rawRows, err := ha.super.Dbw.SqlFetchAll(
		dbTestObserver, row{},
		"SELECT name from environment where id = "+connection.Placeholders(ha.super.Dbw.Db, 0, 1)+" LIMIT 1",
		ha.super.EnvId,
	)

	require.NoError(t, err, "There was an unexpected SQL error")

	rows := rawRows.([]row)

	assert.Equal(t, 1, len(rows), "There should be a row inserted")
	assert.Equal(t, env.Name, rows[0].Name, "name must match")
}

func TestHA_runHA(t *testing.T) {
	ha := createTestingHA(t, testbackends.RedisTestAddr)
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
	ha := createTestingHA(t, testbackends.RedisTestAddr)
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
	ha := createTestingHA(t, testbackends.RedisTestAddr)
	ha.isActive = true
	go ha.StartEventListener()

	testbackends.RedisTestClient.Del("icinga:dump")

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

	testbackends.RedisTestClient.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "host", "state": "done"}})
	testbackends.RedisTestClient.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "host", "state": "wip"}})
	testbackends.RedisTestClient.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "*", "state": "done"}})
	testbackends.RedisTestClient.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "*", "state": "wip"}})
	testbackends.RedisTestClient.XAdd(&redis.XAddArgs{Stream: "icinga:dump", Values: map[string]interface{}{"type": "service", "state": "done"}})

	wg.Wait()
}
