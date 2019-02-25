package icingadb_ha_lib

import (
	"encoding/json"
	"git.icinga.com/icingadb/icingadb-connection-lib"
	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var icingastate = "{\"IcingaApplication\":" +
	"{\"status\": " +
	"{\"icingaapplication\":" +
	"{\"app\":{" +
	"\"environment\": \"\"" +
	"}}}}}"

func TestIcingaEventsBroker(t *testing.T) {
	rd := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:6379",
		DialTimeout:  time.Minute / 2,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
	})

	rdb := icingadb_connection.NewRDBWrapper(rd)

	chEnv := make(chan *icingadb_connection.Environment)

	go func() {
		err := IcingaEventsBroker(rdb, chEnv)
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
