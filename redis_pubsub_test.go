package icingadb_connection

import (
	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestPubSubWrapper(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:6379",
		DialTimeout:  time.Minute / 2,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
	})
	rdbw := NewTestRDBW(rdb)

	if !rdbw.CheckConnection(true) {
		t.Fatal("This test needs a working Redis connection")
	}

	ps := rdbw.Subscribe()

	rdbw.CompareAndSetConnected(false)

	var errSubscribe error
	done1:= make(chan bool)
	go func () {
		errSubscribe = ps.Subscribe("testchannel")
		done1 <- true
	}()

	time.Sleep(50 * time.Millisecond)
	rdbw.CheckConnection(true)

	<- done1

	rdbw.CompareAndSetConnected(false)

	var msg *redis.Message
	var errReceive error
	done2 := make(chan bool)
	go func() {
		msg, errReceive = ps.ReceiveMessage()
		done2 <- true
	}()

	time.Sleep(50 * time.Millisecond)
	rdbw.CheckConnection(true)

	rdbw.Publish("testchannel", "Hello there")

	<- done2

	rdbw.CompareAndSetConnected(false)

	var errClose error
	done3:= make(chan bool)
	go func () {
		errClose = ps.Close()
		done3 <- true
	}()

	time.Sleep(50 * time.Millisecond)
	rdbw.CheckConnection(true)

	<- done3

	assert.NoError(t, errSubscribe)
	assert.NoError(t, errReceive)
	assert.NoError(t, errClose)
	assert.Equal(t, "Hello there", msg.Payload)
}