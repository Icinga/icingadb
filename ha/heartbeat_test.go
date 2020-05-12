// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package ha

import (
	"encoding/json"
	"github.com/Icinga/icingadb/config/testbackends"
	"github.com/Icinga/icingadb/connection"
	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

const app = "{\"status\": " +
	"{\"icingaapplication\":" +
	"{\"app\":{" +
	"\"environment\": \"\"," +
	"\"node_name\": \"master1.icinga.test.com\"" +
	"}}}}"

const dump = "false"

func TestIcingaHeartbeatListener(t *testing.T) {
	rdb := connection.NewRDBWrapper(testbackends.RedisTestAddr, 64)
	assert.True(t, rdb.CheckConnection(), "This test needs a working Redis connection")

	chEnv := make(chan *Environment)

	go func() {
		chErr := make(chan error)
		IcingaHeartbeatListener(rdb, chEnv, chErr)
		require.NoError(t, <-chErr, "redis connection error")
	}()

	time.Sleep(time.Second * 2)

	var uj interface{} = nil
	assert.Nil(t, json.Unmarshal([]byte(app), &uj))
	assert.Nil(t, json.Unmarshal([]byte(dump), &uj))

	rdb.Rdb.XAdd(&redis.XAddArgs{
		Stream: "icinga:stats",
		ID:     "*",
		Values: map[string]interface{}{
			"IcingaApplication":       app,
			"config_dump_in_progress": dump,
		},
	})

	env := <-chEnv

	assert.NotNil(t, env.ID, "no valid env received")
}
