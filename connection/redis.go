package connection

import (
	"fmt"
	"git.icinga.com/icingadb/icingadb-main/utils"
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"sync"
	"sync/atomic"
	"time"
)

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

type RedisClient interface {
	Ping() *redis.StatusCmd
	Publish(channel string, message interface{}) *redis.IntCmd
	XRead(a *redis.XReadArgs) *redis.XStreamSliceCmd
	XDel(stream string, ids ...string) *redis.IntCmd
	HKeys(key string) *redis.StringSliceCmd
	HMGet(key string, fields ...string) *redis.SliceCmd
	HGetAll(key string) *redis.StringStringMapCmd
	TxPipelined(fn func(redis.Pipeliner) error) ([]redis.Cmder, error)
	Pipeline() redis.Pipeliner
	Subscribe(channels ...string) *redis.PubSub
}

type StatusCmd interface {
}

// Redis wrapper including helper functions
type RDBWrapper struct {
	Rdb                         RedisClient
	ConnectedAtomic             *uint32 //uint32 to be able to use atomic operations
	ConnectionUpCondition       *sync.Cond
	ConnectionLostCounterAtomic *uint32 //uint32 to be able to use atomic operations
}

func (rdbw *RDBWrapper) IsConnected() bool {
	return atomic.LoadUint32(rdbw.ConnectedAtomic) != 0
}

func (rdbw *RDBWrapper) CompareAndSetConnected(connected bool) (swapped bool) {
	if connected {
		return atomic.CompareAndSwapUint32(rdbw.ConnectedAtomic, 0, 1)
	} else {
		return atomic.CompareAndSwapUint32(rdbw.ConnectedAtomic, 1, 0)
	}
}

func NewRDBWrapper(address string) (*RDBWrapper, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         address,
		DialTimeout:  time.Minute / 2,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
	})

	rdbw := RDBWrapper{
		Rdb: rdb, ConnectedAtomic: new(uint32),
		ConnectionLostCounterAtomic: new(uint32),
		ConnectionUpCondition:       sync.NewCond(&sync.Mutex{}),
	}

	_, err := rdbw.Rdb.Ping().Result()
	if err != nil {
		return nil, err
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
		v := atomic.LoadUint32(rdbw.ConnectionLostCounterAtomic)
		if v < 4 {
			return 5 * time.Second
		} else if v < 8 {
			return 10 * time.Second
		} else if v < 11 {
			return 30 * time.Second
		} else if v < 14 {
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
			atomic.AddUint32(rdbw.ConnectionLostCounterAtomic, 1)

			log.WithFields(log.Fields{
				"context": "redis",
				"error":   err,
			}).Debugf("Redis connection lost. Trying again in %s", rdbw.getConnectionCheckInterval())
		}

		return false
	} else {
		if rdbw.CompareAndSetConnected(true) {
			log.Info("Redis connection established")
			atomic.StoreUint32(rdbw.ConnectionLostCounterAtomic, 0)
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

// Wrapper for connection handling
func (rdbw *RDBWrapper) HKeys(key string) *redis.StringSliceCmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.HKeys(key)
		_, err := cmd.Result()

		if err != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}

func (rdbw * RDBWrapper) HMGet(key string, fields ...string) *redis.SliceCmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.HMGet(key, fields...)
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
func (rdbw *RDBWrapper) HGetAll(key string) *redis.StringStringMapCmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		benchmarc := utils.NewBenchmark()
		res := rdbw.Rdb.HGetAll(key)

		if _, err := res.Result(); err != nil {
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
			"result":    res.Val(),
		}).Debug("Ran Query")

		return res
	}
}

// Wrapper for auto-logging and connection handling
func (rdbw *RDBWrapper) TxPipelined(fn func(pipeliner redis.Pipeliner) error) ([]redis.Cmder, error) {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		benchmarc := utils.NewBenchmark()
		cmd, err := rdbw.Rdb.TxPipelined(fn)

		if err != nil {
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

		return cmd, err
	}
}

func (rdbw *RDBWrapper) Pipeline() PipelinerWrapper {
	pipeliner := rdbw.Rdb.Pipeline()
	plw := PipelinerWrapper{pipeliner: pipeliner, rdbw: rdbw}
	return plw
}

func (rdbw *RDBWrapper) Subscribe() PubSubWrapper {
	ps := rdbw.Rdb.Subscribe()
	psw := PubSubWrapper{ps: ps, rdbw: rdbw}
	return psw
}

type ConfigChunk struct {
	Keys      []string
	Configs   []interface{}
	Checksums []interface{}
}

type ChecksumChunk struct {
	Keys      []string
	Checksums []interface{}
}

func (rdbw *RDBWrapper) PipeConfigChunks(done <-chan struct{}, keys []string, redisKey string) <-chan *ConfigChunk {
	out := make(chan *ConfigChunk)

	worker := func(chunk <-chan []string) {
		for k := range chunk {
			pipe := rdbw.Pipeline()
			cmds := make([]*redis.SliceCmd, 2)

			cmds[0] = pipe.HMGet(fmt.Sprintf("icinga:config:%s", redisKey), k...)
			cmds[1] = pipe.HMGet(fmt.Sprintf("icinga:checksum:%s", redisKey), k...)

			_, err := pipe.Exec() // TODO(el): What to do with the Cmder slice?
			if err != nil {
				panic(err)
			}

			configs, err := cmds[0].Result()
			if err != nil {
				panic(err)
			}
			checksums, err := cmds[1].Result()
			if err != nil {
				panic(err)
			}

			select {
			case out <- &ConfigChunk{Keys: k, Configs: configs, Checksums: checksums}:
			case <-done:
				return
			}
		}
	}

	//TODO: Replace fixed chunkSize
	work := utils.ChunkKeys(done, keys, 500)

	go func() {
		defer close(out)

		wg := &sync.WaitGroup{}

		for i := 0; i < 32; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				worker(work)
			}()
		}

		wg.Wait()
	}()

	return out
}

func (rdbw *RDBWrapper) PipeChecksumChunks(done <-chan struct{}, keys []string, redisKey string) <-chan *ChecksumChunk {
	out := make(chan *ChecksumChunk)

	worker := func(chunk <-chan []string) {
		for k := range chunk {
			cmd := rdbw.HMGet(fmt.Sprintf("icinga:checksum:%s", redisKey), k...)

			checksums, err := cmd.Result()
			if err != nil {
				panic(err)
			}

			select {
			case out <- &ChecksumChunk{Keys: k, Checksums: checksums}:
			case <-done:
				return
			}
		}
	}

	//TODO: Replace fixed chunkSize
	work := utils.ChunkKeys(done, keys, 500)

	go func() {
		defer close(out)

		wg := &sync.WaitGroup{}

		for i := 0; i < 32; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				worker(work)
			}()
		}

		wg.Wait()
	}()

	return out
}

