package sync

import (
	"encoding/json"
	"fmt"
	"git.icinga.com/icingadb/icingadb-connection"
	"git.icinga.com/icingadb/icingadb-ha"
	"git.icinga.com/icingadb/icingadb-json-decoder"
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/supervisor"
	"git.icinga.com/icingadb/icingadb-utils"
	"git.icinga.com/icingadb/icingadb/benchmark"
	log "github.com/sirupsen/logrus"
	"sync"
	"sync/atomic"
)

// Context provides an Operator with all necessary information to sync a config type
type Context struct {
	ObjectType string
	Factory configobject.RowFactory
	InsertStmt *icingadb_connection.BulkInsertStmt
	DeleteStmt *icingadb_connection.BulkDeleteStmt
	UpdateStmt *icingadb_connection.BulkUpdateStmt
}

type Checksums struct {
	NameChecksum          string  `json:"name_checksum"`
	PropertiesChecksum    string  `json:"properties_checksum"`
	CustomvarsChecksum    string  `json:"customvars_checksum"`
	GroupsChecksum        string  `json:"groups_checksum"`
}

// Operator is the main worker for each config type. It takes a reference to a supervisor super, holding all required
// connection information and other control mechanisms, a channel chHA, which informs the Operator of the current HA
// state, and a Context reference ctx defining the type and providing the necessary factories.
func Operator(super *supervisor.Supervisor, chHA chan int, ctx *Context) error {
	insert, update, delete := GetDelta(super, ctx)
	log.Infof("%s - Delta: (Insert: %d, Maybe Update: %d, Delete: %d)", ctx.ObjectType, len(insert), len(update), len(delete))

	var (
		// If this IcingaDB-Instance looses responsibility, this channel will be
		// closed, resulting in a shutdown of all underlying workers
		done			chan struct{}
		// Used by this Operator to provide the InsertPrepWorker with IDs to insert
		// Operator -> InsertPrepWorker
		chInsert      	chan []string
		// Used by the JsonDecodePool to provide the InsertExecWorker with decoded rows, ready to be inserted
		// JsonDecodePool -> InsertExecWorker
		chInsertBack  	chan []configobject.Row
		// Used by this Operator to provide the DeleteExecWorker with IDs to delete
		// Operator -> DeleteExecWorker
		chDelete      	chan []string
		// Used by this Operator to provide the UpdateCompWorker with IDs to compare
		// Operator -> UpdateCompWorker
		chUpdateComp  	chan []string
		// Used by the UpdateCompWorker to provide the UpdatePrepWorker with IDs that have to be updated
		// UpdateCompWorker -> UpdatePrepWorker
		chUpdate      	chan []string
		// Used by the JsonDecodePool to provide the UpdateExecWorker with decoded rows, ready to be updated
		// JsonDecodePool -> UpdateExecWorker
		chUpdateBack  	chan []configobject.Row
		wgInsert      	*sync.WaitGroup
		wgDelete      	*sync.WaitGroup
		wgUpdate      	*sync.WaitGroup
	)
	for msg := range chHA {
		switch msg {
		// Icinga 2 probably died, stop operations and tell all workers to shut down.
		case icingadb_ha.Notify_StopSync:
			log.Info(fmt.Sprintf("%s: Lost responsibility", ctx.ObjectType))
			if done != nil {
				close(done)
				done = nil
			}
		// Starts up the whole sync process.
		case icingadb_ha.Notify_StartSync:
			log.Infof("%s: Got responsibility", ctx.ObjectType)

			//TODO: This should only be done, if HA was taken over from another instance
			insert, update, delete = GetDelta(super, ctx)
			log.Infof("%s - Delta: (Insert: %d, Maybe Update: %d, Delete: %d)", ctx.ObjectType, len(insert), len(update), len(delete))

			// Clean up all channels and wait groups for a fresh config dump
			done 			= make(chan struct{})
			chInsert      	= make(chan []string)
			chInsertBack  	= make(chan []configobject.Row)
			chDelete      	= make(chan []string)
			chUpdateComp  	= make(chan []string)
			chUpdate      	= make(chan []string)
			chUpdateBack  	= make(chan []configobject.Row)
			wgInsert      	= &sync.WaitGroup{}
			wgDelete      	= &sync.WaitGroup{}
			wgUpdate      	= &sync.WaitGroup{}

			updateCounter := new(uint32)

			go InsertPrepWorker(super, ctx, done, chInsert, chInsertBack)
			go InsertExecWorker(super, ctx, done, chInsertBack, wgInsert)

			go DeleteExecWorker(super, ctx, done, chDelete, wgDelete)

			go UpdateCompWorker(super, ctx, done, chUpdateComp, chUpdate, wgUpdate)
			go UpdatePrepWorker(super, ctx, done, chUpdate, chUpdateBack)
			go UpdateExecWorker(super, ctx, done, chUpdateBack, wgUpdate, updateCounter)

			go func() {
				benchmarc := benchmark.NewBenchmark()
				wgInsert.Add(len(insert))

				// Provide the InsertPrepWorker with IDs to insert
				chInsert <- insert

				// Wait for all IDs to be inserted into MySQL
				wgInsert.Wait()

				benchmarc.Stop()
				log.WithFields(log.Fields{
					"type": 		ctx.ObjectType,
					"count": 		len(insert),
					"benchmark":	benchmarc.String(),
					"action": 		"insert",
				}).Infof("Inserted %v %ss in %v", len(insert), ctx.ObjectType, benchmarc.String())
			}()

			go func() {
				benchmarc := benchmark.NewBenchmark()
				wgDelete.Add(len(delete))

				// Provide the DeleteExecWorker with IDs to delete
				chDelete <- delete

				// Wait for all IDs to be deleted from MySQL
				wgDelete.Wait()

				benchmarc.Stop()
				log.WithFields(log.Fields{
					"type": 		ctx.ObjectType,
					"count": 		len(delete),
					"benchmark":	benchmarc.String(),
					"action": 		"delete",
				}).Infof("Deleted %v %ss in %v", len(delete), ctx.ObjectType, benchmarc.String())
			}()

			go func() {
				benchmarc := benchmark.NewBenchmark()
				wgUpdate.Add(len(update))

				// Provide the UpdateCompWorker with IDs to compare
				chUpdateComp <- update

				// Wait for all IDs to be update in MySQL
				wgUpdate.Wait()

				benchmarc.Stop()
				log.WithFields(log.Fields{
					"type": 		ctx.ObjectType,
					"count": 		atomic.LoadUint32(updateCounter),
					"benchmark":	benchmarc.String(),
					"action": 		"update",
				}).Infof("Updated %v %ss in %v", atomic.LoadUint32(updateCounter), ctx.ObjectType, benchmarc.String())
			}()
		}
	}

	return nil
}

// GetDelta takes a config Context (host, service, checkcommand, etc.) and fetches the ids from MySQL and Redis. It
// returns three string slices:
// 1. IDs which are in the Redis but not in the MySQL (to insert)
// 2. IDs which are in both (to possibly update)
// 3. IDs which are in the MySQL but not the Redis (to delete)
func GetDelta(super *supervisor.Supervisor, ctx *Context) ([]string, []string, []string) {
	var (
		redisIds []string
		mysqlIds []string
		wg = sync.WaitGroup{}
	)

	//get ids from redis
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		res, err := super.Rdbw.HKeys(fmt.Sprintf("icinga:config:checksum:%s", ctx.ObjectType)).Result()
		if err != nil {
			super.ChErr <- err
			return
		}
		redisIds = res
	}()

	//get ids from mysql
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		mysqlIds, err = super.Dbw.SqlFetchIds(ctx.ObjectType)
		if err != nil {
			super.ChErr <- err
			return
		}
	}()

	wg.Wait()
	return icingadb_utils.Delta(redisIds, mysqlIds)
}

// InsertPrepWorker fetches config for IDs(chInsert) from Redis, wraps it into JsonDecodePackages and throws it into the JsonDecodePool
func InsertPrepWorker(super *supervisor.Supervisor, ctx *Context, done chan struct{}, chInsert <-chan []string, chInsertBack chan<- []configobject.Row) {
	defer log.Infof("%s: Insert preparation routine stopped", ctx.ObjectType)

	prep := func(chunk *icingadb_connection.ConfigChunk) {
		pkgs := icingadb_json_decoder.JsonDecodePackages{
			ChBack: chInsertBack,
		}
		for i, key := range chunk.Keys {
			if chunk.Configs[i] == nil || chunk.Checksums[i] == nil {
				continue
			}
			pkg := icingadb_json_decoder.JsonDecodePackage{
				Id:           	key,
				ChecksumsRaw:	chunk.Checksums[i].(string),
				ConfigRaw:   	chunk.Configs[i].(string),
				Factory:		ctx.Factory,
				ObjectType:		ctx.ObjectType,
			}
			pkgs.Packages = append(pkgs.Packages, pkg)
		}

		super.ChDecode <- &pkgs
	}

	for keys := range chInsert {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
		default:
		}

		ch := super.Rdbw.PipeConfigChunks(done, keys, ctx.ObjectType)
		go func() {
			for chunk := range ch {
				go prep(chunk)
			}
		}()
	}
}

// InsertExecWorker gets decoded configobject.Row objects from the JsonDecodePool and inserts them into MySQL
func InsertExecWorker(super *supervisor.Supervisor, ctx *Context, done chan struct{}, chInsertBack <-chan []configobject.Row, wg *sync.WaitGroup) {
	for rows := range chInsertBack {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
		default:
		}

		go func(rows []configobject.Row) {
			super.ChErr <- super.Dbw.SqlBulkInsert(rows, ctx.InsertStmt)
			wg.Add(-len(rows))
		}(rows)
	}
}

// DeleteExecWorker deletes IDs(chDelete) from MySQL
func DeleteExecWorker(super *supervisor.Supervisor, ctx *Context, done chan struct{}, chDelete <-chan []string, wg *sync.WaitGroup) {
	for keys := range chDelete {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
		default:
		}

		go func(keys []string) {
			super.ChErr <- super.Dbw.SqlBulkDelete(keys, ctx.DeleteStmt)
			wg.Add(-len(keys))
		}(keys)
	}
}

// UpdateCompWorker gets IDs(chUpdateComp) that might need an update, fetches the corresponding checksums for Redis and MySQL,
// compares them and inserts changed IDs into chUpdate.
func UpdateCompWorker(super *supervisor.Supervisor, ctx *Context, done chan struct{}, chUpdateComp <-chan []string, chUpdate chan<- []string, wg *sync.WaitGroup) {
	prep := func(chunk *icingadb_connection.ChecksumChunk, mysqlChecksums map[string]map[string]string) {
		changed := make([]string, 0)
		for i, key := range chunk.Keys {
			if chunk.Checksums[i] == nil {
				continue
			}

			//TODO: Check if this can be done better (json should not be processed in this func)
			redisChecksums := &Checksums{}
			err := json.Unmarshal([]byte(chunk.Checksums[i].(string)), redisChecksums)
			if err != nil {
				super.ChErr <- err
			}

			if redisChecksums.PropertiesChecksum != mysqlChecksums[key]["properties_checksum"] {
				changed = append(changed, key)
			} else {
				wg.Done()
			}
		}
		chUpdate <- changed
	}

	for keys := range chUpdateComp {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
		default:
		}

		ch := super.Rdbw.PipeChecksumChunks(done, keys, ctx.ObjectType)
		checksums, err := super.Dbw.SqlFetchChecksums(ctx.ObjectType, keys)
		if err != nil {
			super.ChErr <- err
		}

		go func() {
			for chunk := range ch {
				go prep(chunk, checksums)
			}
		}()
	}
}

// UpdatePrepWorker fetches config for IDs(chUpdate) from Redis, wraps it into JsonDecodePackages and throws it into the JsonDecodePool
func UpdatePrepWorker(super *supervisor.Supervisor, ctx *Context, done chan struct{}, chUpdate <-chan []string, chUpdateBack chan<- []configobject.Row) {
	prep := func(chunk *icingadb_connection.ConfigChunk) {
		pkgs := icingadb_json_decoder.JsonDecodePackages{
			ChBack: chUpdateBack,
		}
		for i, key := range chunk.Keys {
			if chunk.Configs[i] == nil || chunk.Checksums[i] == nil {
				continue
			}
			pkg := icingadb_json_decoder.JsonDecodePackage{
				Id:           	key,
				ChecksumsRaw:	chunk.Checksums[i].(string),
				ConfigRaw:   	chunk.Configs[i].(string),
				Factory:		ctx.Factory,
				ObjectType:		ctx.ObjectType,
			}
			pkgs.Packages = append(pkgs.Packages, pkg)
		}

		super.ChDecode <- &pkgs
	}

	for keys := range chUpdate {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
		default:
		}

		ch := super.Rdbw.PipeConfigChunks(done, keys, ctx.ObjectType)
		go func() {
			for chunk := range ch {
				go prep(chunk)
			}
		}()
	}
}

// UpdateExecWorker gets decoded configobject.Row objects from the JsonDecodePool and updates them in MySQL
func UpdateExecWorker(super *supervisor.Supervisor, ctx *Context, done chan struct{}, chUpdateBack <-chan []configobject.Row, wg *sync.WaitGroup, updateCounter *uint32) {
	for rows := range chUpdateBack {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
		default:
		}

		go func(rows []configobject.Row) {
			super.ChErr <- super.Dbw.SqlBulkUpdate(rows, ctx.UpdateStmt)
			wg.Add(-len(rows))
			atomic.AddUint32(updateCounter, uint32(len(rows)))
		}(rows)
	}
}
