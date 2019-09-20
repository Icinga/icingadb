package ha

import (
	"encoding/json"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var icingastate = "{\"IcingaApplication\":" +
	"{\"status\": " +
	"{\"icingaapplication\":" +
	"{\"app\":{" +
	"\"environment\": \"\"" +
	"}}}}, \"config_dump_in_progress\": false}"

func TestIcingaHeartbeatListener(t *testing.T) {
	rdb := connection.NewRDBWrapper("10.77.27.16:6379")
	assert.True(t, rdb.CheckConnection(false), "This test needs a working Redis connection")

	chEnv := make(chan *Environment)

	go func() {
		err := IcingaHeartbeatListener(rdb, chEnv)
		assert.NoError(t, err, "redis connection error")
	}()

	time.Sleep(time.Second * 2)

	var uj interface{} = nil
	if err := json.Unmarshal([]byte(icingastate), &uj); err != nil {
		assert.Nil(t, err)
	}

	rdb.Rdb.Publish("icinga:stats", icingastate)

	env := <-chEnv

	assert.NotNil(t, env.ID, "no valid env received")
}
