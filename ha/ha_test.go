// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package ha

import (
	"crypto/sha1"
	"github.com/Icinga/icingadb/config/testbackends"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/supervisor"
	"github.com/Icinga/icingadb/utils"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strconv"
	"sync"
	"testing"
	"time"
)

func createTestingHA(t *testing.T, redisAddr string) *HA {
	redisConn := connection.NewRDBWrapper(redisAddr, "", 64)

	mysqlConn, err := connection.NewDBWrapper(testbackends.MysqlTestDsn, 50)
	require.NoError(t, err, "This test needs a working MySQL connection!")

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

func createTestingMultipleHA(t *testing.T, redisAddr string, numInstances int) ([]*HA, <-chan error) {
	redisConn := connection.NewRDBWrapper(redisAddr, "", 64)

	mysqlConn, err := connection.NewDBWrapper(testbackends.MysqlTestDsn, 50)
	require.NoError(t, err, "This test needs a working MySQL connection!")

	_, err = mysqlConn.SqlExec(mysqlTestObserver, "TRUNCATE TABLE icingadb_instance")
	require.NoError(t, err, "This test needs a working MySQL connection!")

	instances := make([]*HA, numInstances)
	chErr := make(chan error)

	for i := 0; i < numInstances; i++ {

		super := supervisor.Supervisor{
			ChErr: chErr,
			Rdbw:  redisConn,
			Dbw:   mysqlConn,
		}

		ha, _ := NewHA(&super)

		hash := sha1.New()
		hash.Write([]byte("derp"))
		ha.super.EnvId = hash.Sum(nil)
		ha.uid = uuid.NewSHA1(uuid.MustParse("551bc748-94b2-4d27-b6a4-15c52aecfe85"), []byte(strconv.Itoa(i)))

		ha.logger = log.WithFields(log.Fields{
			"context": "HA-Testing",
			"UUID":    ha.uid,
		})

		instances[i] = ha
	}

	return instances, chErr
}

var mysqlTestObserver = connection.DbIoSeconds.WithLabelValues("mysql", "test")

func TestHA_UpsertInstance(t *testing.T) {
	ha := createTestingHA(t, testbackends.RedisTestAddr)

	err := ha.super.Dbw.SqlTransaction(true, false, false, func(tx connection.DbTransaction) error {
		return ha.upsertInstance(tx, &Environment{}, false)
	})
	require.NoError(t, err, "transaction running upsertInstance should not return an error")

	rows, err := ha.super.Dbw.SqlFetchAll(
		mysqlObservers.selectIdHeartbeatResponsibleFromIcingadbInstanceByEnvironmentId,
		"SELECT id, heartbeat from icingadb_instance where environment_id = ? LIMIT 1",
		ha.super.EnvId,
	)

	require.NoError(t, err, "There was an unexpected SQL error")
	assert.Equal(t, 1, len(rows), "There should be a row inserted")

	var theirUUID uuid.UUID
	copy(theirUUID[:], rows[0][0].([]byte))

	assert.Equal(t, ha.uid, theirUUID, "UUID must match")
}

func TestHA_checkResponsibility_NoOtherInstance(t *testing.T) {
	ha := createTestingHA(t, testbackends.RedisTestAddr)

	now := utils.TimeToMillisecs(time.Now())
	ha.lastHeartbeat = now
	ha.checkResponsibility(&Environment{})

	assert.Equal(t, StateActive, ha.state, "HA should be active if no other instance exists")
}

func TestHA_checkResponsibility_OtherInactiveInstance(t *testing.T) {
	ha := createTestingHA(t, testbackends.RedisTestAddr)

	now := utils.TimeToMillisecs(time.Now())

	otherUuid, err := uuid.NewRandom()
	assert.NoError(t, err, "UUID generation failed")

	_, err = ha.super.Dbw.SqlExec(
		mysqlObservers.insertIntoIcingadbInstance,
		"INSERT INTO icingadb_instance(id, environment_id, responsible, heartbeat,"+
			" icinga2_version, icinga2_start_time)"+
			" VALUES (?, ?, ?, ?, ?, ?)",
		otherUuid[:], ha.super.EnvId, utils.Bool[false], now, "", 0,
	)
	require.NoError(t, err, "This test needs a working MySQL connection!")

	ha.lastHeartbeat = now
	ha.checkResponsibility(&Environment{})

	assert.Equal(t, StateActive, ha.state, "HA should be active if there is only an inactive instance")
}

func TestHA_checkResponsibility_OtherTimedOutInstance(t *testing.T) {
	ha := createTestingHA(t, testbackends.RedisTestAddr)

	now := utils.TimeToMillisecs(time.Now())
	timedOut := now - heartbeatTimeoutMillisecs

	otherUuid, err := uuid.NewRandom()
	assert.NoError(t, err, "UUID generation failed")

	_, err = ha.super.Dbw.SqlExec(
		mysqlObservers.insertIntoIcingadbInstance,
		"INSERT INTO icingadb_instance(id, environment_id, responsible, heartbeat,"+
			" icinga2_version, icinga2_start_time)"+
			" VALUES (?, ?, ?, ?, ?, ?)",
		otherUuid[:], ha.super.EnvId, utils.Bool[true], timedOut, "", 0,
	)
	require.NoError(t, err, "This test needs a working MySQL connection!")

	ha.lastHeartbeat = now
	ha.checkResponsibility(&Environment{})

	assert.Equal(t, StateActive, ha.state, "HA should be active if another instance is timed out")
}

func TestHA_checkResponsibility_OtherActiveInstance(t *testing.T) {
	ha := createTestingHA(t, testbackends.RedisTestAddr)

	now := utils.TimeToMillisecs(time.Now())

	otherUuid, err := uuid.NewRandom()
	assert.NoError(t, err, "UUID generation failed")

	_, err = ha.super.Dbw.SqlExec(
		mysqlObservers.insertIntoIcingadbInstance,
		"INSERT INTO icingadb_instance(id, environment_id, responsible, heartbeat,"+
			" icinga2_version, icinga2_start_time)"+
			" VALUES (?, ?, ?, ?, ?, ?)",
		otherUuid[:], ha.super.EnvId, utils.Bool[true], now, "", 0,
	)
	require.NoError(t, err, "This test needs a working MySQL connection!")

	ha.lastHeartbeat = now
	ha.checkResponsibility(&Environment{})

	assert.Equal(t, StateOtherActive, ha.state, "HA should not be active if another instance is active")
}

func TestHA_checkResponsibility_Concurrent(t *testing.T) {
	numAttempts := 10
	numConcurrentTakeovers := 2
	failed := false

	for attempt := 0; !failed && attempt < numAttempts; attempt++ {
		wg := sync.WaitGroup{}
		wg.Add(numConcurrentTakeovers)

		haInstances, chErr := createTestingMultipleHA(t, testbackends.RedisTestAddr, numConcurrentTakeovers)
		for _, ha := range haInstances {
			ha.lastHeartbeat = utils.TimeToMillisecs(time.Now())
		}

		for _, ha := range haInstances {
			ha := ha
			go func() {
				defer wg.Done()
				ha.checkResponsibility(&Environment{})
			}()
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

	loop:
		for {
			select {
			case err := <-chErr:
				assert.NoError(t, err, "checkResponsibility() should return no error")
				if err != nil {
					failed = true
				}
			case <-done:
				break loop
			}
		}

		numActive := 0
		for _, ha := range haInstances {
			if ha.state == StateActive {
				numActive++
			}
		}

		assert.Equal(t, 1, numActive, "exactly 1 instance must be active after checkResponsibility() but %d are active", numActive)
		if numActive != 1 {
			failed = true
		}
	}
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
	ha := createTestingHA(t, testbackends.RedisTestAddr)

	env := Environment{
		ID:   utils.EncodeChecksum(utils.Checksum("herp")),
		Name: "herp",
	}

	err := ha.setAndInsertEnvironment(&env)
	require.NoError(t, err, "setAndInsertEnvironment should not return an error")

	rows, err := ha.super.Dbw.SqlFetchAll(
		mysqlTestObserver,
		"SELECT name from environment where id = ? LIMIT 1",
		ha.super.EnvId,
	)

	require.NoError(t, err, "There was an unexpected SQL error")
	assert.Equal(t, 1, len(rows), "There should be a row inserted")
	assert.Equal(t, env.Name, rows[0][0], "name must match")
}

func TestHA_runHA(t *testing.T) {
	ha := createTestingHA(t, testbackends.RedisTestAddr)

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

	ha.runHA(chEnv, &Environment{})

	wg.Wait()
}
