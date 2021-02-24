// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package connection

import (
	"fmt"
	"github.com/Icinga/icingadb/utils"
	"github.com/go-redis/redis/v7"
	"github.com/prometheus/client_golang/prometheus"
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

var redisObservers = struct {
	hgetall prometheus.Observer
	multi   prometheus.Observer
}{
	DbIoSeconds.WithLabelValues("redis", "hgetall"),
	DbIoSeconds.WithLabelValues("redis", "multi"),
}

type RedisClient interface {
	Ping() *redis.StatusCmd
	Publish(channel string, message interface{}) *redis.IntCmd
	XRead(a *redis.XReadArgs) *redis.XStreamSliceCmd
	XDel(stream string, ids ...string) *redis.IntCmd
	XAdd(a *redis.XAddArgs) *redis.StringCmd
	HKeys(key string) *redis.StringSliceCmd
	HScan(key string, cursor uint64, match string, count int64) *redis.ScanCmd
	HMGet(key string, fields ...string) *redis.SliceCmd
	HGetAll(key string) *redis.StringStringMapCmd
	TxPipelined(fn func(redis.Pipeliner) error) ([]redis.Cmder, error)
	Pipeline() redis.Pipeliner
	Subscribe(channels ...string) *redis.PubSub
	Eval(script string, keys []string, args ...interface{}) *redis.Cmd
	EvalSha(sha1 string, keys []string, args ...interface{}) *redis.Cmd
	ScriptExists(hashes ...string) *redis.BoolSliceCmd
	ScriptLoad(script string) *redis.StringCmd
	SAdd(key string, members ...interface{}) *redis.IntCmd
	SRem(key string, members ...interface{}) *redis.IntCmd
	Del(keys ...string) *redis.IntCmd
}

type StatusCmd interface {
}

// RDBWrapper is a redis wrapper including helper functions.
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

func NewRDBWrapper(address string, password string, poolSize int) *RDBWrapper {
	log.Info("Connecting to Redis")

	rdb := redis.NewClient(&redis.Options{
		Addr:         address,
		Password:     password,
		DialTimeout:  time.Minute / 2,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
		PoolTimeout:  time.Minute,
		PoolSize:     poolSize,
	})

	rdbw := RDBWrapper{
		Rdb: rdb, ConnectedAtomic: new(uint32),
		ConnectionLostCounterAtomic: new(uint32),
		ConnectionUpCondition:       sync.NewCond(&sync.Mutex{}),
	}

	_, err := rdbw.Rdb.Ping().Result()
	if err != nil {
		log.WithFields(log.Fields{
			"context": "redis",
			"error":   err,
		}).Error("Could not connect to Redis. Trying again")
	}

	go func() {
		for {
			rdbw.CheckConnection(true)
			time.Sleep(rdbw.getConnectionCheckInterval())
		}
	}()

	return &rdbw
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

// Publish is a wrapper for connection handling.
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

// XRead is a wrapper for connection handling.
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

// XDel is a wrapper for connection handling.
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

// XAdd is a wrapper for connection handling.
func (rdbw *RDBWrapper) XAdd(a *redis.XAddArgs) *redis.StringCmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.XAdd(a)
		_, err := cmd.Result()

		if err != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}

// HKeys is a wrapper for connection handling.
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

// HScan is a wrapper for connection handling.
func (rdbw *RDBWrapper) HScan(key string, cursor uint64, match string, count int64) *redis.ScanCmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.HScan(key, cursor, match, count)
		_, _, err := cmd.Result()

		if err != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}

func (rdbw *RDBWrapper) HMGet(key string, fields ...string) *redis.SliceCmd {
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

// HGetAll is a wrapper for auto-logging and connection handling.
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

		redisObservers.hgetall.Observe(benchmarc.Seconds())

		log.WithFields(log.Fields{
			"context":   "redis",
			"benchmark": benchmarc,
			"query":     "HGETALL " + key,
			"result":    res.Val(),
		}).Debug("Ran Query")

		return res
	}
}

// Eval is a wrapper for connection handling.
func (rdbw *RDBWrapper) Eval(script string, keys []string, args ...interface{}) *redis.Cmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.Eval(script, keys, args...)
		_, err := cmd.Result()

		if err != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}

// EvalSha is a wrapper for connection handling.
func (rdbw *RDBWrapper) EvalSha(sha1 string, keys []string, args ...interface{}) *redis.Cmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.EvalSha(sha1, keys, args...)
		_, err := cmd.Result()

		if err != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}

// ScriptExists is a wrapper for connection handling.
func (rdbw *RDBWrapper) ScriptExists(hashes ...string) *redis.BoolSliceCmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.ScriptExists(hashes...)
		_, err := cmd.Result()

		if err != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}

// ScriptLoad is a wrapper for connection handling.
func (rdbw *RDBWrapper) ScriptLoad(script string) *redis.StringCmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.ScriptLoad(script)
		_, err := cmd.Result()

		if err != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}

// SAdd is a wrapper for connection handling.
func (rdbw *RDBWrapper) SAdd(key string, members ...interface{}) *redis.IntCmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.SAdd(key, members...)
		_, err := cmd.Result()

		if err != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}

// SRem is a wrapper for connection handling.
func (rdbw *RDBWrapper) SRem(key string, members ...interface{}) *redis.IntCmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.SRem(key, members...)
		_, err := cmd.Result()

		if err != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}

// Del is a wrapper for connection handling.
func (rdbw *RDBWrapper) Del(keys ...string) *redis.IntCmd {
	for {
		if !rdbw.IsConnected() {
			rdbw.WaitForConnection()
			continue
		}

		cmd := rdbw.Rdb.Del(keys...)
		_, err := cmd.Result()

		if err != nil {
			if !rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}

// TxPipelined is a wrapper for auto-logging and connection handling.
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

		redisObservers.multi.Observe(benchmarc.Seconds())

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

type PipeConfigChunksFlags uint8

const (
	FetchChecksum PipeConfigChunksFlags = 1 << iota
	FetchConfig
)

func (rdbw *RDBWrapper) PipeConfigChunks(done <-chan struct{}, objType string, inChunk *ConfigChunk,
	fetch PipeConfigChunksFlags) <-chan *ConfigChunk {

	out := make(chan *ConfigChunk)

	fetchChecksum := fetch&FetchChecksum == FetchChecksum
	fetchConfig := fetch&FetchConfig == FetchConfig

	if !fetchChecksum && !fetchConfig {
		// Shortcut just forwarding the chunk if no information needs to be fetched.
		go func() {
			select {
			case out <- inChunk:
			case <-done:
			}
		}()

		return out
	}

	worker := func(chChunk <-chan *ConfigChunk) {
		for chunk := range chChunk {
			pipe := rdbw.Pipeline()
			var cmdChecksum, cmdConfig *redis.SliceCmd

			// Checksums is the first query so that in the worst case, we read an older checksum
			// than config and perform a redundant update than missing an update.
			if fetchChecksum {
				cmdChecksum = pipe.HMGet(fmt.Sprintf("icinga:checksum:%s", objType), chunk.Keys...)
			}
			if fetchConfig {
				cmdConfig = pipe.HMGet(fmt.Sprintf("icinga:config:%s", objType), chunk.Keys...)
			}

			_, err := pipe.Exec() // TODO(el): What to do with the Cmder slice?
			if err != nil {
				panic(err)
			}

			if fetchChecksum && cmdChecksum != nil {
				chunk.Checksums, err = cmdChecksum.Result()
				if err != nil {
					panic(err)
				}
			}
			if fetchConfig && cmdConfig != nil {
				chunk.Configs, err = cmdConfig.Result()
				if err != nil {
					panic(err)
				}
			}

			select {
			case out <- chunk:
			case <-done:
				return
			}
		}
	}

	//TODO: Replace fixed chunkSize
	chunkSize := 500

	work := make(chan *ConfigChunk)
	go func() {
		defer close(work)
		for _, c := range utils.ChunkIndices(len(inChunk.Keys), chunkSize) {
			chunk := &ConfigChunk{Keys: inChunk.Keys[c.Begin:c.End]}
			if inChunk.Configs != nil {
				chunk.Configs = inChunk.Configs[c.Begin:c.End]
			}
			if inChunk.Checksums != nil {
				chunk.Checksums = inChunk.Checksums[c.Begin:c.End]
			}
			select {
			case work <- chunk:
			case <-done:
				return
			}
		}
	}()

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
