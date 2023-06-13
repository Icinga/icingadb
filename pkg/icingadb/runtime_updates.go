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
// Note that Sync must only be called once configuration synchronization has been completed.
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
		deleteEntities := make(chan contracts.Entity, r.redis.Options.XReadCount)

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
			ctx, updateMessages, upsertEntities, upsertedFifo, deleteEntities, deletedFifo,
			structify.MakeMapStructifier(reflect.TypeOf(s.Entity()).Elem(), "json"),
		))

		// upsertEntityFunc returns a closure that is used to upsert the regular Icinga DB entities.
		// The returned func is used to directly start a separate goroutine that selects events
		// sequentially (!allowParallel) from the given chan.
		upsertEntityFunc := func(entities <-chan contracts.Entity) func() error {
			return func() error {
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
					ctx, upsertStmt, upsertCount, sem, entities, com.SplitOnDupId[contracts.Entity], onSuccess...,
				)
			}
		}

		// deleteEntityFunc returns a closure that is used to delete the regular Icinga DB entities
		// based on their ids. The returned func is used to directly start a separate goroutine that
		// selects events sequentially (!allowParallel) from the given chan.
		deleteEntityFunc := func(deleteIds <-chan interface{}) func() error {
			return func() error {
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
			}
		}

		// In order to always get the sla entries written even in case of system errors, we need to process these
		// first. Otherwise, Icinga DB may be stopped after the regular queries have been processed, and deleted
		// from the Redis stream, thus we won't be able to generate sla lifecycle for these entities.
		//
		// The general event process flow looks as follows:
		//     structifyStream()  ->  Reads `upsert` & `delete` events from redis and streams the entities to the
		//                            respective chans `upsertEntities`, `deleteEntities` and waits for `upserted`
		//                            and `deleted` chans (!allowParallel) before consuming the next one from redis.
		// - Start a goroutine that consumes from `upsertEntities` (when the current sync subject is of type checkable,
		//   this bulk inserts into the sla lifecycle table with semaphore 1 and forwards the entities to
		//   the next one, which then inserts the entities into the regular Icinga DB tables). After successfully
		//   upserting the entities, (!allowParallel) they are passed sequentially to the `upserted` stream.
		//
		// - Start another goroutine that consumes from `deleteEntities` concurrently. When the current sync subject is
		//   of type checkable, this performs sla lifecycle updates matching the checkables id and `delete_time` 0. When
		//   there is no tracked `created_at` event for a given checkable, this update is essentially a no-op, but
		//   forwards the entities nonetheless to the next one `updatedSlaLifeCycles`.
		//
		//    - This stage is a no-op for all sla lifecycle that have a `created_at` and `deleted_at` db records, where
		//      all duplicated key errors are ignored with the `INSERT ... IGNORE ON ERROR` mechanism. Nevertheless,
		//      this stage also forwards all entities to the next one. This way we don't need to retrieve data from
		//      the sla_lifecycle table to check whether a `created_at` event has already been recorded for any
		//      given checkable.
		//
		// - Finally the original checkables are deleted from the database and (!allowParallel) they are passed
		//   sequentially to the `deleted` stream.
		switch s.Entity().(type) {
		case *v1.Host, *v1.Service:
			entities := make(chan contracts.Entity, 1)
			g.Go(func() error {
				defer close(entities)

				var counter com.Counter
				defer periodic.Start(ctx, r.logger.Interval(), func(_ periodic.Tick) {
					if count := counter.Reset(); count > 0 {
						r.logger.Infof("Upserted %d %s sla lifecycles", count, s.Name())
					}
				}).Stop()

				sl := v1.NewSlaLifecycle()
				stmt, _ := r.db.BuildInsertIgnoreStmt(sl)

				// Not to mess up the already existing FIFO mechanism, we have to perform only a single query
				// (semaphore 1) at a time, even the sla queries could be bulk executed.
				// After successfully upserting a lifecycle entity, the original checkable entity is streamed to "entities".
				slaEntities := CreateSlaLifecyclesFromCheckables(ctx, g, r.db, upsertEntities, false)
				return r.db.NamedBulkExec(
					ctx, stmt, upsertCount, semaphore.NewWeighted(1), slaEntities, com.NeverSplit[contracts.Entity],
					OnSuccessApplyAndSendTo(entities, GetCheckableFromSlaLifecycle), OnSuccessIncrement[contracts.Entity](&counter),
				)
			})

			// Start the regular Icinga DB checkables upsert stream.
			g.Go(upsertEntityFunc(entities))

			deletedIds := make(chan interface{}, r.redis.Options.XReadCount)
			g.Go(func() error {
				defer close(deletedIds)

				var counter com.Counter
				defer periodic.Start(ctx, r.logger.Interval(), func(_ periodic.Tick) {
					if count := counter.Reset(); count > 0 {
						r.logger.Infof("Upserting %d %s sla lifecycles", count, s.Name())
					}
				}).Stop()

				// extractEntityId is used as a callback for the on success mechanism to extract the checkables id.
				extractEntityId := func(e contracts.Entity) interface{} {
					return e.(*v1.SlaLifecycle).SourceEntity.ID()
				}

				sl := v1.NewSlaLifecycle()
				stmt, _ := r.db.BuildInsertIgnoreStmt(sl)
				sem := r.db.GetSemaphoreForTable(utils.TableName(sl))

				updatedSlaLifeCycles := UpdateSlaLifeCycles(
					ctx, r.db, deleteEntities, g, upsertCount, r.redis.Options.XReadCount, OnSuccessIncrement[contracts.Entity](&counter),
				)

				return r.db.NamedBulkExec(
					ctx, stmt, upsertCount, sem, updatedSlaLifeCycles, com.NeverSplit[contracts.Entity],
					OnSuccessApplyAndSendTo[contracts.Entity, interface{}](deletedIds, extractEntityId),
				)
			})

			// Start the regular Icinga DB checkables delete stream.
			g.Go(deleteEntityFunc(deletedIds))
		default:
			// For non-checkables runtime updates of upsert event
			g.Go(upsertEntityFunc(upsertEntities))

			// For non-checkables runtime updates of delete event
			deleteIds := make(chan interface{}, r.redis.Options.XReadCount)
			g.Go(func() error {
				defer close(deleteIds)

				for {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case entity, ok := <-deleteEntities:
						if !ok {
							return nil
						}

						select {
						case deleteIds <- entity.ID():
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				}
			})

			g.Go(deleteEntityFunc(deleteIds))
		}
	}

	// customvar and customvar_flat sync.
	{
		updateMessages := make(chan redis.XMessage, r.redis.Options.XReadCount)
		upsertEntities := make(chan contracts.Entity, r.redis.Options.XReadCount)
		deleteEntities := make(chan contracts.Entity, r.redis.Options.XReadCount)

		cv := common.NewSyncSubject(v1.NewCustomvar)
		cvFlat := common.NewSyncSubject(v1.NewCustomvarFlat)

		r.logger.Debug("Syncing runtime updates of " + cv.Name())
		r.logger.Debug("Syncing runtime updates of " + cvFlat.Name())

		updateMessagesByKey["icinga:"+utils.Key(cv.Name(), ':')] = updateMessages
		g.Go(structifyStream(
			ctx, updateMessages, upsertEntities, nil, deleteEntities, nil,
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
				case _, ok := <-deleteEntities:
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
			rs, err := r.redis.XReadUntilResult(ctx, &redis.XReadArgs{
				Streams: streams.Option(),
				Count:   int64(r.redis.Options.XReadCount),
			})
			if err != nil {
				return errors.Wrap(err, "can't read runtime updates")
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
	deleteEntities chan contracts.Entity, deleted chan interface{}, structifier structify.MapStructifier,
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
			close(deleteEntities)
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
					case deleteEntities <- entity:
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
