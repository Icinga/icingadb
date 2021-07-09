package icingadb

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/contracts"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/structify"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"reflect"
)

// RuntimeUpdates specifies the source and destination of runtime updates.
type RuntimeUpdates struct {
	db     *DB
	redis  *icingaredis.Client
	logger *zap.SugaredLogger
}

// NewRuntimeUpdates creates a new RuntimeUpdates.
func NewRuntimeUpdates(db *DB, redis *icingaredis.Client, logger *zap.SugaredLogger) *RuntimeUpdates {
	return &RuntimeUpdates{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

const bulkSize = 1 << 14

// Sync synchronizes Redis runtime streams from s.redis to s.db and deletes the original data on success.
// Note that Sync must be only be called configuration synchronization has been completed.
func (r *RuntimeUpdates) Sync(ctx context.Context, factoryFuncs []contracts.EntityFactoryFunc, lastStreamId string) error {
	g, ctx := errgroup.WithContext(ctx)

	stream := "icinga:runtime"
	updateMessagesByKey := make(map[string]chan<- redis.XMessage)

	for _, factoryFunc := range factoryFuncs {
		factoryFunc = factoryFunc.WithInit

		updateMessages := make(chan redis.XMessage, bulkSize)
		upsertEntities := make(chan contracts.Entity, bulkSize)
		deleteIds := make(chan interface{}, bulkSize)

		v := factoryFunc()
		name := utils.Name(v)

		updateMessagesByKey[fmt.Sprintf("icinga:%s", utils.Key(name, ':'))] = updateMessages

		r.logger.Debugf("Syncing runtime updates of %s", name)
		g.Go(structifyStream(ctx, updateMessages, upsertEntities, deleteIds, structify.MakeMapStructifier(reflect.TypeOf(v).Elem(), "json")))

		g.Go(func() error {
			stmt, placeholders := r.db.BuildUpsertStmt(v)
			// Updates must be executed in order, ensure this by using a semaphore with maximum 1.
			sem := semaphore.NewWeighted(1)
			return r.db.NamedBulkExec(ctx, stmt, 1<<15/placeholders, sem, upsertEntities, nil)
		})
		g.Go(func() error {
			return r.db.DeleteStreamed(ctx, v, deleteIds)
		})
	}

	{
		updateMessages := make(chan redis.XMessage, bulkSize)
		upsertEntities := make(chan contracts.Entity, bulkSize)
		deleteIds := make(chan interface{}, bulkSize)

		var cvFactoryFunc contracts.EntityFactoryFunc
		cvFactoryFunc = v1.NewCustomvar
		cvFactoryFunc = cvFactoryFunc.WithInit
		cv := cvFactoryFunc()
		name := utils.Name(cv)

		r.logger.Debug("Syncing runtime updates of " + name)
		r.logger.Debug("Syncing runtime updates of CustomvarFlat")

		updateMessagesByKey["icinga:"+utils.Key(name, ':')] = updateMessages

		g.Go(structifyStream(ctx, updateMessages, upsertEntities, deleteIds, structify.MakeMapStructifier(reflect.TypeOf(cv).Elem(), "json")))

		customvars, flatCustomvars, errs := v1.ExpandCustomvars(ctx, upsertEntities)
		com.ErrgroupReceive(g, errs)

		g.Go(func() error {
			stmt, _ := r.db.BuildUpsertStmt(cv)
			// Updates must be executed in order, ensure this by using a semaphore with maximum 1.
			sem := semaphore.NewWeighted(1)
			// TODO(el) Currently not possible to increase the count here: https://github.com/jmoiron/sqlx/issues/694
			return r.db.NamedBulkExec(ctx, stmt, 1, sem, customvars, nil)
		})

		g.Go(func() error {
			stmt, _ := r.db.BuildUpsertStmt(v1.NewCustomvarFlat())
			// Updates must be executed in order, ensure this by using a semaphore with maximum 1.
			sem := semaphore.NewWeighted(1)
			// TODO(el) Currently not possible to increase the count here: https://github.com/jmoiron/sqlx/issues/694
			return r.db.NamedBulkExec(ctx, stmt, 1, sem, flatCustomvars, nil)
		})

		deleteCustomvars, deleteFlatCustomvars := make(chan interface{}), make(chan interface{})
		g.Go(func() error {
			defer close(deleteCustomvars)
			defer close(deleteFlatCustomvars)
			for {
				select {
				case id, ok := <-deleteIds:
					if !ok {
						return nil
					}

					deleteCustomvars <- id
					deleteFlatCustomvars <- id
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		g.Go(func() error {
			return r.db.DeleteStreamed(ctx, cv, deleteCustomvars)
		})

		var cvFlatFactoryFunc contracts.EntityFactoryFunc
		cvFlatFactoryFunc = v1.NewCustomvarFlat
		cvFlatFactoryFunc = cvFactoryFunc.WithInit
		cvFlat := cvFlatFactoryFunc()

		g.Go(func() error {
			return r.db.Semaphored(cvFlat, func(sem *semaphore.Weighted) error {
				return r.db.BulkExec(ctx, `DELETE FROM customvar_flat WHERE customvar_id IN (?)`, 1<<15, sem, deleteFlatCustomvars)
			})
		})
	}

	g.Go(r.xRead(ctx, updateMessagesByKey, stream, lastStreamId))

	return g.Wait()
}

// xRead reads from the Redis stream and sends the data to the corresponding updateMessages channel.
// The updateMessages channel is determined by a "redis_key" on each redis message.
func (r *RuntimeUpdates) xRead(ctx context.Context, updateMessagesByKey map[string]chan<- redis.XMessage, stream string, lastStreamId string) func() error {
	return func() error {
		defer func() {
			for _, updateMessages := range updateMessagesByKey {
				close(updateMessages)
			}
		}()

		for {
			xra := &redis.XReadArgs{
				Streams: []string{stream, lastStreamId},
				Count:   bulkSize,
				Block:   0,
			}

			cmd := r.redis.XRead(ctx, xra)
			streams, err := cmd.Result()

			if err != nil {
				return icingaredis.WrapCmdErr(cmd)
			}

			for _, stream := range streams {
				for _, message := range stream.Messages {
					lastStreamId = message.ID

					redisKey := message.Values["redis_key"]
					if redisKey == nil {
						return errors.Errorf("stream message missing 'redis_key' key: %v", message.Values)
					}

					updateMessages := updateMessagesByKey[redisKey.(string)]
					if updateMessages == nil {
						return errors.Errorf("no object type for redis key %s found", redisKey)
					}

					select {
					case updateMessages <- message:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
		}
	}
}

// structifyStream gets Redis stream messages (redis.XMessage) via the updateMessages channel and converts
// those messages into Icinga DB entities (contracts.Entity) using the provided structifier.
// Converted entities are inserted into the upsertEntities or deleteIds channel depending on the "runtime_type" message field.
func structifyStream(ctx context.Context, updateMessages <-chan redis.XMessage, upsertEntities chan contracts.Entity, deleteIds chan interface{}, structifier structify.MapStructifier) func() error {
	return func() error {
		defer func() {
			close(upsertEntities)
			close(deleteIds)
		}()

		for {
			select {
			case message, ok := <-updateMessages:
				if !ok {
					return nil
				}

				ptr, err := structifier(message.Values)
				if err != nil {
					return errors.Wrapf(err, "can't structify values %#v", message.Values)
				}

				entity := ptr.(contracts.Entity)

				runtimeType := message.Values["runtime_type"]
				if runtimeType == nil {
					return errors.Errorf("stream message missing 'runtime_type' key: %v", message.Values)
				}

				if runtimeType == "upsert" {
					select {
					case upsertEntities <- entity:
					case <-ctx.Done():
						return ctx.Err()
					}
				} else if runtimeType == "delete" {
					select {
					case deleteIds <- entity.ID():
					case <-ctx.Done():
						return ctx.Err()
					}
				} else {
					return errors.Errorf("invalid runtime type: %s", runtimeType)
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
