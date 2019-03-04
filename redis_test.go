package icingadb_connection

import (
	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func NewTestRDBW(rdb RedisClient) RDBWrapper {
	dbw := RDBWrapper{Rdb: rdb, ConnectedAtomic: new(uint32), ConnectionLostCounterAtomic: new(uint32)}
	dbw.ConnectionUpCondition = sync.NewCond(&sync.Mutex{})
	return dbw
}

func TestNewRDBWrapper(t *testing.T) {
	_, err := NewRDBWrapper("127.0.0.1:6379")
	assert.NoError(t, err, "Redis should be connected")

	_, err = NewRDBWrapper("asdasdasdasdasd:5123")
	assert.Error(t, err, "Redis should not be connected")
	//TODO: Add more tests here
}

func TestRDBWrapper_GetConnectionCheckInterval(t *testing.T) {
	rdbw := NewTestRDBW(nil)

	//Should return 15s, if connected - counter doesn't madder
	rdbw.CompareAndSetConnected(true)
	assert.Equal(t, 15*time.Second, rdbw.getConnectionCheckInterval())

	//Should return 5s, if not connected and counter < 4
	rdbw.CompareAndSetConnected(false)
	atomic.StoreUint32(rdbw.ConnectionLostCounterAtomic, 0)
	assert.Equal(t, 5*time.Second, rdbw.getConnectionCheckInterval())

	//Should return 10s, if not connected and 4 <= counter < 8
	rdbw.CompareAndSetConnected(false)
	atomic.StoreUint32(rdbw.ConnectionLostCounterAtomic, 4)
	assert.Equal(t, 10*time.Second, rdbw.getConnectionCheckInterval())

	//Should return 30s, if not connected and 8 <= counter < 11
	rdbw.CompareAndSetConnected(false)
	atomic.StoreUint32(rdbw.ConnectionLostCounterAtomic, 8)
	assert.Equal(t, 30*time.Second, rdbw.getConnectionCheckInterval())

	//Should return 60s, if not connected and 11 <= counter < 14
	rdbw.CompareAndSetConnected(false)
	atomic.StoreUint32(rdbw.ConnectionLostCounterAtomic, 11)
	assert.Equal(t, 60*time.Second, rdbw.getConnectionCheckInterval())

	//dbw.ConnectionLostCounter = 14
	//interval = dbw.getConnectionCheckInterval()
	//TODO: Check for Fatal
}

func TestRDBWrapper_CheckConnection(t *testing.T) {
	rdbw := NewTestRDBW(nil)

	rdbw.Rdb = redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:6379",
		DialTimeout:  time.Minute / 2,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
	})
	atomic.StoreUint32(rdbw.ConnectionLostCounterAtomic, 512312312)
	assert.True(t, rdbw.CheckConnection(false), "DBWrapper should be connected")
	assert.Equal(t, uint32(0), atomic.LoadUint32(rdbw.ConnectionLostCounterAtomic))

	rdbw.Rdb = redis.NewClient(&redis.Options{
		Addr:         "dasdasdasdasdasd:5123",
		DialTimeout:  time.Minute / 2,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
	})
	atomic.StoreUint32(rdbw.ConnectionLostCounterAtomic, 0)
	assert.False(t, rdbw.CheckConnection(false), "DBWrapper should not be connected")
	assert.Equal(t, uint32(0), atomic.LoadUint32(rdbw.ConnectionLostCounterAtomic))

	atomic.StoreUint32(rdbw.ConnectionLostCounterAtomic, 10)
	assert.False(t, rdbw.CheckConnection(true), "DBWrapper should not be connected")
	assert.Equal(t, uint32(11), atomic.LoadUint32(rdbw.ConnectionLostCounterAtomic))
}

func TestRDBWrapper_HGetAll(t *testing.T) {
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

	rdb.Del("herpdaderp")
	rdb.HSet("herpdaderp", "one", 5)
	rdb.HSet("herpdaderp", "two", 11)

	rdbw.CompareAndSetConnected(false)

	var data map[string]string
	var err error
	done := make(chan bool)
	go func() {
		data, err = rdbw.HGetAll("herpdaderp")
		done <- true
	}()

	time.Sleep(500 * time.Millisecond)
	rdbw.CheckConnection(true)

	<- done

	assert.NoError(t, err)
	assert.Contains(t, data, "one")
	assert.Contains(t, data, "two")
}

func TestRDBWrapper_XRead(t *testing.T) {
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

	rdb.XTrim("teststream", 0)
	rdb.XAdd(&redis.XAddArgs{Stream: "teststream", Values: map[string]interface{}{"one": "5", "two": "11", "herp": "11"}})

	rdbw.CompareAndSetConnected(false)

	var data *redis.XStreamSliceCmd
	done := make(chan bool)
	go func() {
		data = rdbw.XRead(&redis.XReadArgs{Streams: []string{"teststream", "0"}})
		done <- true
	}()

	time.Sleep(500 * time.Millisecond)
	rdbw.CheckConnection(true)

	<- done

	streams, err := data.Result()
	assert.NoError(t, err)
	value := streams[0].Messages[0].Values

	assert.Contains(t, value, "one")
	assert.Contains(t, value, "two")
}

func TestRDBWrapper_XDel(t *testing.T) {
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

	rdb.XTrim("teststream", 0)
	adds := rdb.XAdd(&redis.XAddArgs{Stream: "teststream", Values: map[string]interface{}{"one": "5", "two": "11", "herp": "11"}})

	rdbw.CompareAndSetConnected(false)

	done := make(chan bool)
	go func() {
		rdbw.XDel("teststream", adds.Val())
		done <- true
	}()

	time.Sleep(500 * time.Millisecond)
	rdbw.CheckConnection(true)

	<- done

	data := rdbw.XRead(&redis.XReadArgs{Streams: []string{"teststream", "0"}, Block: -1})
	streams, err := data.Result()
	assert.Error(t, err)
	assert.Len(t, streams, 0)
}
