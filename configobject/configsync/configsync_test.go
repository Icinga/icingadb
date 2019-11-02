package configsync

import (
	"fmt"
	"github.com/go-redis/redis"
	"github.com/icinga/icingadb/configobject"
	"github.com/icinga/icingadb/configobject/objecttypes/host"
	"github.com/icinga/icingadb/connection"
	"github.com/icinga/icingadb/connection/mysqld"
	"github.com/icinga/icingadb/connection/redisd"
	"github.com/icinga/icingadb/ha"
	"github.com/icinga/icingadb/jsondecoder"
	"github.com/icinga/icingadb/supervisor"
	"github.com/icinga/icingadb/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

var mysqlTestObserver = connection.DbIoSeconds.WithLabelValues("mysql", "test")

func SetupConfigSync(t *testing.T, objectTypes []*configobject.ObjectInformation) (*supervisor.Supervisor, []chan int, *redisd.Server, *redis.Client, *mysqld.Server) {
	var redisServer redisd.Server

	redisClient, errSrv := redisServer.Start()
	if errSrv != nil {
		t.Fatal(errSrv)
	}

	var mysqlServer mysqld.Server

	host, errSt := mysqlServer.Start()
	if errSt != nil {
		t.Fatal(errSt)
	}

	if errMTD := mysqld.MkTestDb(host); errMTD != nil {
		t.Fatal(errMTD)
	}

	rdbw := connection.NewRDBWrapper(redisClient.Options().Addr, 64)
	dbw, err := connection.NewDBWrapper(fmt.Sprintf("icingadb:icingadb@%s/icingadb", host), 50)
	require.NoError(t, err, "Is the MySQL server running?")

	super := supervisor.Supervisor{
		ChErr:    make(chan error),
		ChDecode: make(chan *jsondecoder.JsonDecodePackages),
		Rdbw:     rdbw,
		Dbw:      dbw,
		EnvLock:  &sync.Mutex{},
		EnvId:    utils.EncodeChecksum("e057d4ea363fbab414a874371da253dba3d713bc"),
	}

	go jsondecoder.DecodePool(super.ChDecode, super.ChErr, 16)

	chs := make([]chan int, 0)

	for _, objectInformation := range objectTypes {
		ch := make(chan int)
		chs = append(chs, ch)

		go func(information *configobject.ObjectInformation, ch chan int) {
			super.ChErr <- Operator(&super, ch, information)
		}(objectInformation, ch)
	}

	return &super, chs, &redisServer, redisClient, &mysqlServer
}

func TearDownConfigSync(redisServer *redisd.Server, redisClient *redis.Client, mysqlServer *mysqld.Server) {
	redisServer.Stop()
	redisClient.Close()
	mysqlServer.Stop()
}

func TestOperator_InsertHost(t *testing.T) {
	super, chs, redisServer, redisClient, mysqlServer := SetupConfigSync(t, []*configobject.ObjectInformation{
		&host.ObjectInformation,
	})
	defer TearDownConfigSync(redisServer, redisClient, mysqlServer)

	redisClient.Del("icinga:config:host")
	redisClient.Del("icinga:checksum:host")

	_, err := super.Dbw.Db.Exec("TRUNCATE TABLE host")
	require.NoError(t, err)

	redisClient.HSet("icinga:config:host", "a9ef44eb69fda8fbc32bee33322b6518057f559f", "{\"active_checks_enabled\":false,\"address\":\"localhost\",\"address6\":\"\",\"check_interval\":10.0,\"check_retry_interval\":60.0,\"check_timeout\":null,\"checkcommand\":\"dummy\",\"checkcommand_id\":\"adc77319f261b771b35ce671aaf956d3c7534808\",\"display_name\":\"TestHost - 603\",\"environment_id\":\""+utils.DecodeChecksum(super.EnvId)+"\",\"event_handler_enabled\":true,\"flapping_enabled\":false,\"flapping_threshold_high\":30.0,\"flapping_threshold_low\":25.0,\"icon_image_alt\":\"\",\"is_volatile\":false,\"max_check_attempts\":3.0,\"name\":\"TestHost - 603\",\"name_checksum\":\"8ae04eb17df433de95fb6b855464e393f3d6ef72\",\"notes\":\"\",\"notifications_enabled\":true,\"passive_checks_enabled\":true,\"perfdata_enabled\":true}")
	redisClient.HSet("icinga:checksum:host", "a9ef44eb69fda8fbc32bee33322b6518057f559f", "{\"checksum\":\"b6e87de3d4f31b3d4d35466171f4088693b46071\"}")

	for _, ch := range chs {
		ch <- ha.Notify_StartSync
	}

	assert.Eventually(t, func() bool {
		objects, err := super.Dbw.SqlFetchAll(mysqlTestObserver, "SELECT * FROM host")
		require.NoError(t, err)

		if len(objects) == 1 && utils.DecodeChecksum(objects[0][3].([]byte)) == "b6e87de3d4f31b3d4d35466171f4088693b46071" {
			require.Equal(t, "TestHost - 603", objects[0][8], "display_name should be set to 'TestHost - 603'")
			require.Equal(t, "localhost", objects[0][9], "address should be set to 'localhost'")
			require.Equal(t, "dummy", objects[0][13], "check_command should be set to 'dummy'")
			return true
		} else {
			return false
		}
	}, 3*time.Second, 1*time.Second, "Exactly 1 host should be synced")
}

func TestOperator_DeleteHost(t *testing.T) {
	super, chs, redisServer, redisClient, mysqlServer := SetupConfigSync(t, []*configobject.ObjectInformation{
		&host.ObjectInformation,
	})
	defer TearDownConfigSync(redisServer, redisClient, mysqlServer)

	redisClient.Del("icinga:config:host")
	redisClient.Del("icinga:checksum:host")

	_, err := super.Dbw.Db.Exec("TRUNCATE TABLE host")
	require.NoError(t, err)

	someChecksum := utils.EncodeChecksum(utils.Checksum("some_checksum"))

	_, err = super.Dbw.Db.Exec(
		"INSERT INTO host(id, environment_id, name_checksum, properties_checksum, customvars_checksum, groups_checksum, name, name_ci, display_name, address, address6, address_bin, address6_bin, checkcommand, checkcommand_id, max_check_attempts, check_timeperiod, check_timeperiod_id, check_timeout, check_interval, check_retry_interval, active_checks_enabled, passive_checks_enabled, event_handler_enabled, notifications_enabled, flapping_enabled, flapping_threshold_low, flapping_threshold_high, perfdata_enabled, eventcommand, eventcommand_id, is_volatile, action_url_id, notes_url_id, notes, icon_image_id, icon_image_alt, zone, zone_id, command_endpoint, command_endpoint_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		someChecksum,
		super.EnvId,
		someChecksum,
		someChecksum,
		someChecksum,
		someChecksum,
		"name",
		"name_ci",
		"display_name",
		"address",
		"address6",
		[]byte{255, 255, 255, 255},
		[]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		"checkcommand",
		someChecksum,
		10,
		"check_timeperiod",
		someChecksum,
		0,
		0,
		0,
		"y",
		"y",
		"y",
		"y",
		"y",
		0,
		0,
		"y",
		"eventcommand",
		someChecksum,
		"y",
		someChecksum,
		someChecksum,
		"notes",
		someChecksum,
		"icon_image_alt",
		"zone",
		someChecksum,
		"command_endpoint",
		someChecksum,
	)
	require.NoError(t, err)

	for _, ch := range chs {
		ch <- ha.Notify_StartSync
	}

	assert.Eventually(t, func() bool {
		objects, err := super.Dbw.SqlFetchAll(mysqlTestObserver, "SELECT * FROM host")
		require.NoError(t, err)

		return len(objects) == 0
	}, 3*time.Second, 1*time.Second, "Exactly 1 host should be deleted")
}

func TestOperator_UpdateHost(t *testing.T) {
	super, chs, redisServer, redisClient, mysqlServer := SetupConfigSync(t, []*configobject.ObjectInformation{
		&host.ObjectInformation,
	})
	defer TearDownConfigSync(redisServer, redisClient, mysqlServer)

	redisClient.Del("icinga:config:host")
	redisClient.Del("icinga:checksum:host")

	_, err := super.Dbw.Db.Exec("TRUNCATE TABLE host")
	require.NoError(t, err)

	redisClient.HSet("icinga:config:host", "a9ef44eb69fda8fbc32bee33322b6518057f559f", "{\"active_checks_enabled\":false,\"address\":\"localhost\",\"address6\":\"\",\"check_interval\":10.0,\"check_retry_interval\":60.0,\"check_timeout\":null,\"checkcommand\":\"dummy\",\"checkcommand_id\":\"adc77319f261b771b35ce671aaf956d3c7534808\",\"display_name\":\"TestHost - 603\",\"environment_id\":\""+utils.DecodeChecksum(super.EnvId)+"\",\"event_handler_enabled\":true,\"flapping_enabled\":false,\"flapping_threshold_high\":30.0,\"flapping_threshold_low\":25.0,\"icon_image_alt\":\"\",\"is_volatile\":false,\"max_check_attempts\":3.0,\"name\":\"TestHost - 603\",\"name_checksum\":\"8ae04eb17df433de95fb6b855464e393f3d6ef72\",\"notes\":\"\",\"notifications_enabled\":true,\"passive_checks_enabled\":true,\"perfdata_enabled\":true}")
	redisClient.HSet("icinga:checksum:host", "a9ef44eb69fda8fbc32bee33322b6518057f559f", "{\"checksum\":\"b6e87de3d4f31b3d4d35466171f4088693b46071\"}")

	someChecksum := utils.EncodeChecksum(utils.Checksum("some_checksum"))

	_, err = super.Dbw.Db.Exec(
		"INSERT INTO host(id, environment_id, name_checksum, properties_checksum, customvars_checksum, groups_checksum, name, name_ci, display_name, address, address6, address_bin, address6_bin, checkcommand, checkcommand_id, max_check_attempts, check_timeperiod, check_timeperiod_id, check_timeout, check_interval, check_retry_interval, active_checks_enabled, passive_checks_enabled, event_handler_enabled, notifications_enabled, flapping_enabled, flapping_threshold_low, flapping_threshold_high, perfdata_enabled, eventcommand, eventcommand_id, is_volatile, action_url_id, notes_url_id, notes, icon_image_id, icon_image_alt, zone, zone_id, command_endpoint, command_endpoint_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		utils.EncodeChecksum("a9ef44eb69fda8fbc32bee33322b6518057f559f"),
		super.EnvId,
		someChecksum,
		someChecksum,
		someChecksum,
		someChecksum,
		"name",
		"name_ci",
		"display_name",
		"address",
		"address6",
		[]byte{255, 255, 255, 255},
		[]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		"checkcommand",
		someChecksum,
		10,
		"check_timeperiod",
		someChecksum,
		0,
		0,
		0,
		"y",
		"y",
		"y",
		"y",
		"y",
		0,
		0,
		"y",
		"eventcommand",
		someChecksum,
		"y",
		someChecksum,
		someChecksum,
		"notes",
		someChecksum,
		"icon_image_alt",
		"zone",
		someChecksum,
		"command_endpoint",
		someChecksum,
	)
	require.NoError(t, err)

	for _, ch := range chs {
		ch <- ha.Notify_StartSync
	}

	assert.Eventually(t, func() bool {
		objects, err := super.Dbw.SqlFetchAll(mysqlTestObserver, "SELECT * FROM host")
		require.NoError(t, err)

		if len(objects) > 0 && utils.DecodeChecksum(objects[0][3].([]byte)) == "b6e87de3d4f31b3d4d35466171f4088693b46071" {
			require.Equal(t, "TestHost - 603", objects[0][8], "display_name should be set to 'TestHost - 603'")
			require.Equal(t, "localhost", objects[0][9], "address should be set to 'localhost'")
			require.Equal(t, "dummy", objects[0][13], "check_command should be set to 'dummy'")
			require.Equal(t, 1, len(objects), "There should only be 1 host in the Database")
			return true
		} else {
			return false
		}
	}, 3*time.Second, 1*time.Second, "Exactly 1 host should be synced")
}
