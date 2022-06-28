package icingadb

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/icingaredis/telemetry"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/periodic"
	"github.com/icinga/icingadb/pkg/structify"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// RuntimeUpdates specifies the source and destination of runtime updates.
type RuntimeUpdates struct {
	db     *DB
	redis  *icingaredis.Client
	logger *logging.Logger
}

// NewRuntimeUpdates creates a new RuntimeUpdates.
func NewRuntimeUpdates(db *DB, redis *icingaredis.Client, logger *logging.Logger) *RuntimeUpdates {
	return &RuntimeUpdates{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

// ClearStreams returns the stream key to ID mapping of the runtime update streams
// for later use in Sync and clears the streams themselves.
func (r *RuntimeUpdates) ClearStreams(ctx context.Context) (config, state icingaredis.Streams, err error) {
	config = icingaredis.Streams{"icinga:runtime": "0-0"}
	state = icingaredis.Streams{"icinga:runtime:state": "0-0"}

	var keys []string
	for _, streams := range [...]icingaredis.Streams{config, state} {
		for key := range streams {
			keys = append(keys, key)
		}
	}

	err = icingaredis.WrapCmdErr(r.redis.Del(ctx, keys...))
	return
}

// Sync synchronizes runtime update streams from s.redis to s.db and deletes the original data on success.
// Note that Sync must be only be called configuration synchronization has been completed.
// allowParallel allows synchronizing out of order (not FIFO).
func (r *RuntimeUpdates) Sync(
	ctx context.Context, factoryFuncs []contracts.EntityFactoryFunc, streams icingaredis.Streams, allowParallel bool,
) error {
	g, ctx := errgroup.WithContext(ctx)

	updateMessagesByKey := make(map[string]chan<- redis.XMessage)

	for _, factoryFunc := range factoryFuncs {
		s := common.NewSyncSubject(factoryFunc)
		stat := getCounterForEntity(s.Entity())

		updateMessages := make(chan redis.XMessage, r.redis.Options.XReadCount)
		upsertEntities := make(chan contracts.Entity, r.redis.Options.XReadCount)
		deleteIds := make(chan interface{}, r.redis.Options.XReadCount)

		var upsertedFifo chan contracts.Entity
		var deletedFifo chan interface{}
		var upsertCount int
		var deleteCount int
		upsertStmt, upsertPlaceholders := r.db.BuildUpsertStmt(s.Entity())
		if !allowParallel {
			upsertedFifo = make(chan contracts.Entity, 1)
			deletedFifo = make(chan interface{}, 1)
			upsertCount = 1
			deleteCount = 1
		} else {
			upsertCount = r.db.BatchSizeByPlaceholders(upsertPlaceholders)
			deleteCount = r.db.Options.MaxPlaceholdersPerStatement
		}

		updateMessagesByKey[fmt.Sprintf("icinga:%s", utils.Key(s.Name(), ':'))] = updateMessages

		r.logger.Debugf("Syncing runtime updates of %s", s.Name())

		g.Go(structifyStream(
			ctx, updateMessages, upsertEntities, upsertedFifo, deleteIds, deletedFifo,
			structify.MakeMapStructifier(reflect.TypeOf(s.Entity()).Elem(), "json"),
		))

		g.Go(func() error {
			var counter com.Counter
			defer periodic.Start(ctx, r.logger.Interval(), func(_ periodic.Tick) {
				if count := counter.Reset(); count > 0 {
					r.logger.Infof("Upserted %d %s items", count, s.Name())
				}
			}).Stop()

			// Updates must be executed in order, ensure this by using a semaphore with maximum 1.
			sem := semaphore.NewWeighted(1)

			onSuccess := []OnSuccess[contracts.Entity]{
				OnSuccessIncrement[contracts.Entity](&counter), OnSuccessIncrement[contracts.Entity](stat),
			}
			if !allowParallel {
				onSuccess = append(onSuccess, OnSuccessSendTo(upsertedFifo))
			}

			return r.db.NamedBulkExec(
				ctx, upsertStmt, upsertCount, sem, upsertEntities, com.SplitOnDupId[contracts.Entity], onSuccess...,
			)
		})

		g.Go(func() error {
			var counter com.Counter
			defer periodic.Start(ctx, r.logger.Interval(), func(_ periodic.Tick) {
				if count := counter.Reset(); count > 0 {
					r.logger.Infof("Deleted %d %s items", count, s.Name())
				}
			}).Stop()

			sem := r.db.GetSemaphoreForTable(utils.TableName(s.Entity()))

			onSuccess := []OnSuccess[any]{OnSuccessIncrement[any](&counter), OnSuccessIncrement[any](stat)}
			if !allowParallel {
				onSuccess = append(onSuccess, OnSuccessSendTo(deletedFifo))
			}

			return r.db.BulkExec(ctx, r.db.BuildDeleteStmt(s.Entity()), deleteCount, sem, deleteIds, onSuccess...)
		})
	}

	// customvar and customvar_flat sync.
	{
		updateMessages := make(chan redis.XMessage, r.redis.Options.XReadCount)
		upsertEntities := make(chan contracts.Entity, r.redis.Options.XReadCount)
		deleteIds := make(chan interface{}, r.redis.Options.XReadCount)

		cv := common.NewSyncSubject(v1.NewCustomvar)
		cvFlat := common.NewSyncSubject(v1.NewCustomvarFlat)

		r.logger.Debug("Syncing runtime updates of " + cv.Name())
		r.logger.Debug("Syncing runtime updates of " + cvFlat.Name())

		updateMessagesByKey["icinga:"+utils.Key(cv.Name(), ':')] = updateMessages
		g.Go(structifyStream(
			ctx, updateMessages, upsertEntities, nil, deleteIds, nil,
			structify.MakeMapStructifier(reflect.TypeOf(cv.Entity()).Elem(), "json"),
		))

		customvars, flatCustomvars, errs := v1.ExpandCustomvars(ctx, upsertEntities)
		com.ErrgroupReceive(g, errs)

		cvStmt, cvPlaceholders := r.db.BuildUpsertStmt(cv.Entity())
		cvCount := r.db.BatchSizeByPlaceholders(cvPlaceholders)
		g.Go(func() error {
			var counter com.Counter
			defer periodic.Start(ctx, r.logger.Interval(), func(_ periodic.Tick) {
				if count := counter.Reset(); count > 0 {
					r.logger.Infof("Upserted %d %s items", count, cv.Name())
				}
			}).Stop()

			// Updates must be executed in order, ensure this by using a semaphore with maximum 1.
			sem := semaphore.NewWeighted(1)

			return r.db.NamedBulkExec(
				ctx, cvStmt, cvCount, sem, customvars, com.SplitOnDupId[contracts.Entity],
				OnSuccessIncrement[contracts.Entity](&counter),
				OnSuccessIncrement[contracts.Entity](&telemetry.Stats.Config),
			)
		})

		cvFlatStmt, cvFlatPlaceholders := r.db.BuildUpsertStmt(cvFlat.Entity())
		cvFlatCount := r.db.BatchSizeByPlaceholders(cvFlatPlaceholders)
		g.Go(func() error {
			var counter com.Counter
			defer periodic.Start(ctx, r.logger.Interval(), func(_ periodic.Tick) {
				if count := counter.Reset(); count > 0 {
					r.logger.Infof("Upserted %d %s items", count, cvFlat.Name())
				}
			}).Stop()

			// Updates must be executed in order, ensure this by using a semaphore with maximum 1.
			sem := semaphore.NewWeighted(1)

			return r.db.NamedBulkExec(
				ctx, cvFlatStmt, cvFlatCount, sem, flatCustomvars,
				com.SplitOnDupId[contracts.Entity], OnSuccessIncrement[contracts.Entity](&counter),
				OnSuccessIncrement[contracts.Entity](&telemetry.Stats.Config),
			)
		})

		g.Go(func() error {
			var once sync.Once
			for {
				select {
				case _, ok := <-deleteIds:
					if !ok {
						return nil
					}
					// Icinga 2 does not send custom var delete events.
					once.Do(func() {
						r.logger.DPanic("received unexpected custom var delete event")
					})
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})
	}

	g.Go(r.xRead(ctx, updateMessagesByKey, streams))

	return g.Wait()
}

// xRead reads from the runtime update streams and sends the data to the corresponding updateMessages channel.
// The updateMessages channel is determined by a "redis_key" on each redis message.
func (r *RuntimeUpdates) xRead(ctx context.Context, updateMessagesByKey map[string]chan<- redis.XMessage, streams icingaredis.Streams) func() error {
	return func() error {
		defer func() {
			for _, updateMessages := range updateMessagesByKey {
				close(updateMessages)
			}
		}()

		for {
			xra := &redis.XReadArgs{
				Streams: streams.Option(),
				Count:   int64(r.redis.Options.XReadCount),
				Block:   0,
			}

			cmd := r.redis.XRead(ctx, xra)
			rs, err := cmd.Result()

			if err != nil {
				return icingaredis.WrapCmdErr(cmd)
			}

			pipe := r.redis.Pipeline()
			for _, stream := range rs {
				var id string

				for _, message := range stream.Messages {
					id = message.ID

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

				tsAndSerial := strings.Split(id, "-")
				if s, err := strconv.ParseUint(tsAndSerial[1], 10, 64); err == nil {
					tsAndSerial[1] = strconv.FormatUint(s+1, 10)
				}

				pipe.XTrimMinIDApprox(ctx, stream.Stream, strings.Join(tsAndSerial, "-"), 0)
				streams[stream.Stream] = id
			}

			if cmds, err := pipe.Exec(ctx); err != nil {
				r.logger.Errorw("Can't execute Redis pipeline", zap.Error(errors.WithStack(err)))
			} else {
				for _, cmd := range cmds {
					if cmd.Err() != nil {
						r.logger.Errorw("Can't trim runtime updates stream", zap.Error(icingaredis.WrapCmdErr(cmd)))
					}
				}
			}
		}
	}
}

// structifyStream gets Redis stream messages (redis.XMessage) via the updateMessages channel and converts
// those messages into Icinga DB entities (contracts.Entity) using the provided structifier.
// Converted entities are inserted into the upsertEntities or deleteIds channel depending on the "runtime_type" message field.
func structifyStream(
	ctx context.Context, updateMessages <-chan redis.XMessage, upsertEntities, upserted chan contracts.Entity,
	deleteIds, deleted chan interface{}, structifier structify.MapStructifier,
) func() error {
	if upserted == nil {
		upserted = make(chan contracts.Entity)
		close(upserted)
	}

	if deleted == nil {
		deleted = make(chan interface{})
		close(deleted)
	}

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

					select {
					case <-upserted:
					case <-ctx.Done():
						return ctx.Err()
					}
				} else if runtimeType == "delete" {
					select {
					case deleteIds <- entity.ID():
					case <-ctx.Done():
						return ctx.Err()
					}

					select {
					case <-deleted:
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
