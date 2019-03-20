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

func Operator(super *supervisor.Supervisor, chHA chan int, ctx *Context) error {
	insert, update, delete := GetDelta(super, ctx)
	log.Infof("%s - Delta: (Insert: %d, Maybe Update: %d, Delete: %d)", ctx.ObjectType, len(insert), len(update), len(delete))

	var (
		done			chan struct{}
		chInsert      	= make(chan []string)
		chInsertBack  	= make(chan []configobject.Row)
		chDelete      	= make(chan []string)
		chUpdateComp  	= make(chan []string)
		chUpdate      	= make(chan []string)
		chUpdateBack  	= make(chan []configobject.Row)
		wgInsert      	= &sync.WaitGroup{}
		wgDelete      	= &sync.WaitGroup{}
		wgUpdate      	= &sync.WaitGroup{}
	)
	go func() {
		for msg := range chHA {
			switch msg {
			case icingadb_ha.Notify_IsNotResponsible:
				log.Info(fmt.Sprintf("%s: Lost responsibility", ctx.ObjectType))
				if done != nil {
					close(done)
				}
			case icingadb_ha.Notify_IsResponsible:
				log.Infof("%s: Got responsibility", ctx.ObjectType)

				//TODO: This should only be done, if HA was taken over from another instance
				insert, update, delete = GetDelta(super, ctx)

				done = make(chan struct{})
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
					chInsert <- insert
					wgInsert.Wait()
					benchmarc.Stop()
					log.Infof("Inserted %v %ss in %v", len(insert), ctx.ObjectType, benchmarc.String())
				}()

				go func() {
					benchmarc := benchmark.NewBenchmark()
					wgDelete.Add(len(delete))
					chDelete <- delete
					wgDelete.Wait()
					benchmarc.Stop()
					log.Infof("Deleted %v %ss in %v", len(delete), ctx.ObjectType, benchmarc.String())
				}()

				go func() {
					benchmarc := benchmark.NewBenchmark()
					wgUpdate.Add(len(update))
					chUpdateComp <- update
					wgUpdate.Wait()
					benchmarc.Stop()
					log.Infof("Updated %v %ss in %v", atomic.LoadUint32(updateCounter), ctx.ObjectType, benchmarc.String())
				}()
			}
		}
	}()

	return nil
}

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

func InsertPrepWorker(super *supervisor.Supervisor, ctx *Context, done chan struct{}, chInsert <-chan []string, chInsertBack chan<- []configobject.Row) {
	defer log.Infof("%s: Insert preparation routine stopped", ctx.ObjectType)

	prep := func(chunk *icingadb_connection.ConfigChunk) {
		pkgs := icingadb_json_decoder.JsonDecodePackages{
			ChBack: chInsertBack,
		}
		for i, key := range chunk.Keys {
			select {
			case _, ok := <-done:
				if !ok {
					return
				}
			default:
			}

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

func UpdateCompWorker(super *supervisor.Supervisor, ctx *Context, done chan struct{}, chUpdate <-chan []string, chUpdateBack chan<- []string, wg *sync.WaitGroup) {
	prep := func(chunk *icingadb_connection.ChecksumChunk, mysqlChecksums map[string]map[string]string) {
		changed := make([]string, 0)
		for i, key := range chunk.Keys {
			select {
			case _, ok := <-done:
				if !ok {
					return
				}
			default:
			}

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
		chUpdateBack <- changed
	}

	for keys := range chUpdate {
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
				select {
				case _, ok := <-done:
					if !ok {
						return
					}
				default:
				}

				go prep(chunk, checksums)
			}
		}()
	}
}

func UpdatePrepWorker(super *supervisor.Supervisor, ctx *Context, done chan struct{}, chUpdate <-chan []string, chUpdateBack chan<- []configobject.Row) {
	prep := func(chunk *icingadb_connection.ConfigChunk) {
		pkgs := icingadb_json_decoder.JsonDecodePackages{
			ChBack: chUpdateBack,
		}
		for i, key := range chunk.Keys {
			select {
			case _, ok := <-done:
				if !ok {
					return
				}
			default:
			}

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
				select {
				case _, ok := <-done:
					if !ok {
						return
					}
				default:
				}

				go prep(chunk)
			}
		}()
	}
}

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
