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
	"}}}}}"

func TestIcingaEventsBroker(t *testing.T) {
	rdb, err := connection.NewRDBWrapper("127.0.0.1:6379")
	if err != nil {
		t.Fatal("This test needs a working Redis connection")
	}

	chEnv := make(chan *Environment)

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
