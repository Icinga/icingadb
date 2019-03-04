package icingadb_connection

import (
	"git.icinga.com/icingadb/icingadb-utils-lib"
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"sync"
	"sync/atomic"
	"time"
)

type Environment struct {
	ID   []byte
	Name string
}

type Icinga2RedisWriterEventsConfig struct {
	Update, Delete, Dump string
}

type Icinga2RedisWriterKeyPrefixesConfig struct {
	Checksum, Object, Customvar string
}

type Icinga2RedisWriterKeyPrefixesStatus struct {
	Object string
}

type Icinga2RedisWriterEvents struct {
	Config Icinga2RedisWriterEventsConfig
	Stats  string
}

type Icinga2RedisWriterKeyPrefixes struct {
	Config Icinga2RedisWriterKeyPrefixesConfig
	Status Icinga2RedisWriterKeyPrefixesStatus
}

type Icinga2RedisWriter struct {
	Events      Icinga2RedisWriterEvents
	KeyPrefixes Icinga2RedisWriterKeyPrefixes
}

var RedisWriter = Icinga2RedisWriter{
	Events: Icinga2RedisWriterEvents{
		Config: Icinga2RedisWriterEventsConfig{
			Dump:   "icinga:config:dump",
			Delete: "icinga:config:delete",
			Update: "icinga:config:update",
		},
		Stats: "icinga:stats",
	},
	KeyPrefixes: Icinga2RedisWriterKeyPrefixes{
		Config: Icinga2RedisWriterKeyPrefixesConfig{
			Checksum:  "icinga:config:checksum:",
			Object:    "icinga:config:object:",
			Customvar: "icinga:config:customvar:",
		},
		Status: Icinga2RedisWriterKeyPrefixesStatus{
			Object: "icinga:state:object:",
		},
	},
}

// Redis wrapper including helper functions
type RDBWrapper struct {
	Rdb                    		*redis.Client
	ConnectedAtomic        		*uint32 //uint32 to be able to use atomic operations
	ConnectionUpCondition 		*sync.Cond
	ConnectionLostCounter   	int
}

func (rdbw *RDBWrapper) IsConnected() bool {
	return *rdbw.ConnectedAtomic != 0
}

func (rdbw *RDBWrapper) CompareAndSetConnected(connected bool) (swapped bool) {
	if connected {
		return atomic.CompareAndSwapUint32(rdbw.ConnectedAtomic, 0, 1)
	} else {
		return atomic.CompareAndSwapUint32(rdbw.ConnectedAtomic, 1, 0)
	}
}

func NewRDBWrapper(rdb *redis.Client) (*RDBWrapper, error) {
	rdbw := RDBWrapper{Rdb: rdb, ConnectedAtomic: new(uint32)}
	rdbw.ConnectionUpCondition = sync.NewCond(&sync.Mutex{})

	res := rdbw.Rdb.Ping()
	if res.Err() != nil {
		return nil, res.Err()
	}

	go func() {
		for {
			rdbw.CheckConnection(true)
			time.Sleep(rdbw.getConnectionCheckInterval())
		}
	}()

	return &rdbw, nil
}

func (rdbw *RDBWrapper) getConnectionCheckInterval() time.Duration {
	if !rdbw.IsConnected() {
		if rdbw.ConnectionLostCounter < 4 {
			return 5 * time.Second
		} else if rdbw.ConnectionLostCounter < 8 {
			return 10 * time.Second
		} else if rdbw.ConnectionLostCounter < 11 {
			return 30 * time.Second
		} else if rdbw.ConnectionLostCounter < 14 {
			return 60 * time.Second
		} else {
			log.Fatal("Could not connect to Redis for over 5 minutes. Shutting down...")
		}
	}

	return 15 * time.Second
}

func (rdbw *RDBWrapper) CheckConnection(isTicker bool) bool {
	_, err := rdbw.Rdb.Ping().Result()
	if err != nil {
		if rdbw.CompareAndSetConnected(false) {
			log.WithFields(log.Fields{
				"context": "redis",
				"error":   err,
			}).Error("Redis connection lost. Trying to reconnect")
		} else if isTicker {
			rdbw.ConnectionLostCounter++

			log.WithFields(log.Fields{
				"context": "redis",
				"error":   err,
			}).Debugf("Redis connection lost. Trying again in %s", rdbw.getConnectionCheckInterval())
		}

		return false
	} else {
		if rdbw.CompareAndSetConnected(true) {
			log.Info("Redis connection established")
			rdbw.ConnectionLostCounter = 0
			rdbw.ConnectionUpCondition.Broadcast()
		}

		return true
	}
}

func (rdbw *RDBWrapper) WaitForConnection() {
	rdbw.ConnectionUpCondition.L.Lock()
	rdbw.ConnectionUpCondition.Wait()
	rdbw.ConnectionUpCondition.L.Unlock()
}

// Wrapper for connection handling
func (rdbw *RDBWrapper) Publish(channel string, message interface{}) *redis.IntCmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.Publish(channel, message)
		_, err := cmd.Result()

		if err != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}

// Wrapper for connection handling
func (rdbw *RDBWrapper) XRead(args *redis.XReadArgs) *redis.XStreamSliceCmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.XRead(args)
		_, err := cmd.Result()

		if err != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}
// Wrapper for connection handling
func (rdbw *RDBWrapper) XDel(stream string, ids ...string) *redis.IntCmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.XDel(stream, ids...)
		_, err := cmd.Result()

		if err != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}

// Wrapper for auto-logging and connection handling
func (rdbw *RDBWrapper) HGetAll(key string) (map[string]string, error) {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		benchmarc := icingadb_utils.NewBenchmark()
		res, errHGA := rdbw.Rdb.HGetAll(key).Result()

		if errHGA != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		benchmarc.Stop()

		DbIoSeconds.WithLabelValues("redis", "hgetall").Observe(benchmarc.Seconds())

		log.WithFields(log.Fields{
			"context":   "redis",
			"benchmark": benchmarc,
			"query":     "HGETALL " + key,
			"result":    res,
		}).Debug("Ran Query")

		return res, errHGA
	}
}

// Wrapper for auto-logging and connection handling
func (rdbw *RDBWrapper) TxPipelined(fn func(pipeliner redis.Pipeliner) error) ([]redis.Cmder, error) {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
		continue
		}
		benchmarc := icingadb_utils.NewBenchmark()
		c, e := rdbw.Rdb.TxPipelined(fn)

		if e != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		benchmarc.Stop()

		DbIoSeconds.WithLabelValues("redis", "multi").Observe(benchmarc.Seconds())

		log.WithFields(log.Fields{
			"context":   "redis",
			"benchmark": benchmarc,
			"query":     "MULTI/EXEC",
		}).Debug("Ran pipelined transaction")

		return c, e
	}
}

func (rdbw *RDBWrapper) Subscribe() PubSubWrapper {
	ps := rdbw.Rdb.Subscribe()
	psw := PubSubWrapper{ps: ps, rdbw: rdbw}
	return psw
}