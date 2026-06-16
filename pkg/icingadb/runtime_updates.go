package icingadb

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/structify"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis/telemetry"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"maps"
	"reflect"
	"sync"
)

// RuntimeUpdates specifies the source and destination of runtime updates.
type RuntimeUpdates struct {
	db     *database.DB
	redis  *redis.Client
	logger *logging.Logger
}

// NewRuntimeUpdates creates a new RuntimeUpdates.
func NewRuntimeUpdates(db *database.DB, redis *redis.Client, logger *logging.Logger) *RuntimeUpdates {
	return &RuntimeUpdates{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

// ClearStreams returns the stream key to ID mapping of the runtime update streams
// for later use in Sync and clears the streams themselves.
func (r *RuntimeUpdates) ClearStreams(ctx context.Context) (config, state redis.Streams, err error) {
	config = redis.Streams{"icinga:runtime": "0-0"}
	state = redis.Streams{"icinga:runtime:state": "0-0"}

	var keys []string
	for _, streams := range [...]redis.Streams{config, state} {
		for key := range streams {
			keys = append(keys, key)
		}
	}

	err = redis.WrapCmdErr(r.redis.Del(ctx, keys...))
	return
}

// prepareCustomVarsForSync prepares the channels and goroutines for synchronizing custom variables.
//
// The returned channel is the one to which the Redis stream messages for custom variables will be sent.
func (r *RuntimeUpdates) prepareCustomVarsForSync(ctx context.Context, g *errgroup.Group) chan<- redis.XMessage {
	updateMessages := make(chan redis.XMessage, r.redis.Options.XReadCount)
	upsertEntities := make(chan database.Entity, r.redis.Options.XReadCount)
	deleteIds := make(chan any, r.redis.Options.XReadCount)

	cv := common.NewSyncSubject(v1.NewCustomvar)
	cvFlat := common.NewSyncSubject(v1.NewCustomvarFlat)

	r.logger.Debug("Syncing runtime updates of " + cv.Name())
	r.logger.Debug("Syncing runtime updates of " + cvFlat.Name())

	g.Go(structifyStream(
		ctx, updateMessages, upsertEntities, deleteIds, nil,
		structify.MakeMapStructifier(
			reflect.TypeOf(cv.Entity()).Elem(),
			"json",
			contracts.SafeInit),
	))

	customvars, flatCustomvars, errs := v1.ExpandCustomvars(ctx, upsertEntities)
	com.ErrgroupReceive(g, errs)

	syncableCvs := map[*common.SyncSubject]<-chan database.Entity{cv: customvars, cvFlat: flatCustomvars}
	for s, cvInCh := range syncableCvs {
		g.Go(func() error {
			var counter com.Counter
			defer periodic.Start(ctx, r.logger.Interval(), func(_ periodic.Tick) {
				if count := counter.Reset(); count > 0 {
					r.logger.Infof("Upserted %d %s items", count, s.Name())
				}
			}).Stop()

			// Updates must be executed in order, ensure this by using a semaphore with maximum 1.
			sem := semaphore.NewWeighted(1)

			stmt, placeholders := r.db.BuildUpsertStmt(s.Entity())
			return r.db.NamedBulkExec(
				ctx, stmt, r.db.BatchSizeByPlaceholders(placeholders), sem, cvInCh,
				database.SplitOnDupId[database.Entity],
				database.OnSuccessIncrement[database.Entity](&counter),
				database.OnSuccessIncrement[database.Entity](&telemetry.Stats.Config),
			)
		})
	}

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

	return updateMessages
}

// Sync synchronizes runtime update streams from r.redis to r.db and deletes the original data on success.
//
// The options parameter can be used to specify additional options for the synchronization, such as allowing
// parallel execution of updates for the same entity type (bulk DB upsert and delete ops) or providing a callback
// that is called for each Redis stream message of type "upsert" in parallel with the main database upsert operations
// for the same entity type.
//
// Note that Sync must only be called after configuration synchronization has been completed.
func (r *RuntimeUpdates) Sync(
	ctx context.Context,
	factoryFuncs []database.EntityFactoryFunc,
	streams redis.Streams,
	options ...RUOption,
) error {
	if len(streams) != 1 {
		return errors.New("streams must contain exactly one stream key for the runtime updates")
	}

	type messageByKey map[string]chan<- redis.XMessage
	// xReads is a slice of length 2, where the first element is the map of each entity keys to channels for the
	// main sync, and the second element is the map for the additional synchronization with custom onUpsert callback
	// (if specified). This separation is necessary because we want to start a separate xRead goroutine for each sync,
	// and each xRead goroutine needs its own set of channels to send the messages to.
	xReads := make([]messageByKey, 2)

	opts := new(RUOptions)
	for _, opt := range options {
		opt(opts)
	}

	g, ctx := errgroup.WithContext(ctx)
	prepareForSync := func(s *common.SyncSubject, serializerCh <-chan any) (chan<- redis.XMessage, <-chan database.Entity, <-chan any) {
		var upsertEntities chan database.Entity
		var updateMessages chan redis.XMessage
		var deleteIds chan any

		// Don't use buffered channels if a serializer channel is provided, because the structifyStream goroutine will
		// block on the serializer channel after each message dispatch to the respective upsertEntities or deleteIds
		// channel. So, we don't need to unnecessarily use huge buffers in that case!
		if serializerCh == nil {
			updateMessages = make(chan redis.XMessage, r.redis.Options.XReadCount)
			upsertEntities = make(chan database.Entity, r.redis.Options.XReadCount)
			deleteIds = make(chan any, r.redis.Options.XReadCount)
		} else {
			updateMessages = make(chan redis.XMessage)
			upsertEntities = make(chan database.Entity)
			deleteIds = make(chan any)
		}

		g.Go(structifyStream(
			ctx, updateMessages, upsertEntities, deleteIds, serializerCh,
			structify.MakeMapStructifier(
				reflect.TypeOf(s.Entity()).Elem(),
				"json",
				contracts.SafeInit),
		))
		return updateMessages, upsertEntities, deleteIds
	}

	for _, factoryFunc := range factoryFuncs {
		s := common.NewSyncSubject(factoryFunc)
		stat := getCounterForEntity(s.Entity())

		r.logger.Debugf("Syncing runtime updates of %s", s.Name())

		key := fmt.Sprintf("icinga:%s", strcase.Delimited(s.Name(), ':'))

		if opts.upsertFn != nil {
			r.logger.Debugf("Starting additional sync with custom onUpsert callback for %s", s.Name())

			serializerCh := make(chan any)
			updateMessages, upsertEntities, deleteIds := prepareForSync(s, serializerCh)
			if xReads[1] == nil {
				xReads[1] = make(messageByKey)
			}
			xReads[1][key] = updateMessages

			g.Go(func() error {
				defer close(serializerCh)

				for {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case entity, ok := <-upsertEntities:
						if !ok {
							return nil
						}
						if err := opts.upsertFn(ctx, entity); err != nil {
							return errors.Wrapf(err, "onUpsert callback failed for entity with ID %v", entity.ID())
						}
					case _, ok := <-deleteIds:
						if !ok {
							return nil
						}
						// Nothing to do for deletes in the onUpsert callback, but we need to consume
						// the channel to avoid blocking the structifyStream goroutine.
					}

					select {
					case <-ctx.Done():
						return ctx.Err()
					case serializerCh <- struct{}{}:
					}
				}
			})
		}

		var serializerCh chan any
		if !opts.allowParallel {
			serializerCh = make(chan any)
		}

		updateMessages, upsertEntities, deleteIds := prepareForSync(s, serializerCh)
		if xReads[0] == nil {
			xReads[0] = make(messageByKey)
		}
		xReads[0][key] = updateMessages

		g.Go(func() error {
			var counter com.Counter
			defer periodic.Start(ctx, r.logger.Interval(), func(_ periodic.Tick) {
				if count := counter.Reset(); count > 0 {
					r.logger.Infof("Upserted %d %s items", count, s.Name())
				}
			}).Stop()

			// Updates must be executed in order, ensure this by using a semaphore with maximum 1.
			sem := semaphore.NewWeighted(1)

			onSuccess := []database.OnSuccess[database.Entity]{
				database.OnSuccessIncrement[database.Entity](&counter), database.OnSuccessIncrement[database.Entity](stat),
			}

			var upsertCount int
			upsertStmt, upsertPlaceholders := r.db.BuildUpsertStmt(s.Entity())
			if !opts.allowParallel {
				upsertCount = 1
				onSuccess = append(onSuccess, database.OnSuccessApplyAndSendTo(serializerCh, func(e database.Entity) any { return e }))
			} else {
				upsertCount = r.db.BatchSizeByPlaceholders(upsertPlaceholders)
			}

			return r.db.NamedBulkExec(
				ctx, upsertStmt, upsertCount, sem, upsertEntities, database.SplitOnDupId[database.Entity], onSuccess...,
			)
		})

		g.Go(func() error {
			var counter com.Counter
			defer periodic.Start(ctx, r.logger.Interval(), func(_ periodic.Tick) {
				if count := counter.Reset(); count > 0 {
					r.logger.Infof("Deleted %d %s items", count, s.Name())
				}
			}).Stop()

			sem := r.db.GetSemaphoreForTable(database.TableName(s.Entity()))

			onSuccess := []database.OnSuccess[any]{database.OnSuccessIncrement[any](&counter), database.OnSuccessIncrement[any](stat)}
			var deleteCount int
			if !opts.allowParallel {
				deleteCount = 1
				onSuccess = append(onSuccess, database.OnSuccessSendTo(serializerCh))
			} else {
				deleteCount = r.db.Options.MaxPlaceholdersPerStatement
			}

			return r.db.BulkExec(ctx, r.db.BuildDeleteStmt(s.Entity()), deleteCount, sem, deleteIds, onSuccess...)
		})
	}

	// customvar and customvar_flat sync don't need to be processed with state updates too.
	if _, exists := streams["icinga:runtime"]; exists {
		if xReads[0] == nil {
			xReads[0] = make(messageByKey)
		}
		xReads[0]["icinga:"+strcase.Delimited(types.Name(v1.Customvar{}), ':')] = r.prepareCustomVarsForSync(ctx, g)
	}

	// Since all xRead goroutines are going to consume messages from the same stream independently, we are only
	// allowed to send XDel commands after we've successfully dispatched all messages to the corresponding
	// updateMessages channels. For the database ops, it doesn't really matter when the msgs are acked, but for
	// the onUpsert callback, the per type updates are processed sequentially, so the xRead will block each time
	// it tries to send a message to the chOuts channel until the callback has processed the previous one. When
	// the callback fails to process the previous one (which we already deleted from the stream), we'll either
	// trigger a HA handover or crash Icinga DB fatally, in which case losing that message is not a big deal.
	var xRedisMessageAcks []<-chan string
	for _, chOuts := range xReads {
		if chOuts == nil {
			continue
		}
		ackMessageCh := make(chan string)
		xRedisMessageAcks = append(xRedisMessageAcks, ackMessageCh)

		g.Go(r.xRead(ctx, chOuts, ackMessageCh, maps.Clone(streams)))
	}

	g.Go(func() error { return r.redis.XDelOnAllConsumersAck(ctx, streams.Option()[0], xRedisMessageAcks...) })

	return g.Wait()
}

// xRead reads from the runtime update streams and sends the data to the corresponding updateMessages channel.
// The updateMessages channel is determined by a "redis_key" on each redis message.
func (r *RuntimeUpdates) xRead(
	ctx context.Context,
	updateMessagesByKey map[string]chan<- redis.XMessage,
	acknowledgementOutCh chan<- string,
	streams redis.Streams,
) func() error {
	return func() error {
		defer func() {
			close(acknowledgementOutCh)
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

			for _, stream := range rs {
				var id string

				for _, message := range stream.Messages {
					id = message.ID

					redisKey, ok := message.Values["redis_key"].(string)
					if !ok {
						return errors.Errorf("redis_key stream message key is %T, not string", message.Values["redis_key"])
					}

					updateMessages := updateMessagesByKey[redisKey]
					if updateMessages == nil {
						return errors.Errorf("no object type for redis key %s found", redisKey)
					}

					select {
					case updateMessages <- message:
					case <-ctx.Done():
						return ctx.Err()
					}

					select {
					case acknowledgementOutCh <- message.ID:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				streams[stream.Stream] = id
			}
		}
	}
}

// structifyStream gets Redis stream messages (redis.XMessage) via the updateMessages channel and converts
// those messages into Icinga DB entities (contracts.Entity) using the provided structifier.
// Converted entities are inserted into the upsertEntities or deleteIds channel depending on the "runtime_type" message field.
func structifyStream(
	ctx context.Context,
	messagesInCh <-chan redis.XMessage,
	upsertEntitiesOutCh chan<- database.Entity,
	deleteIdsOutCh chan<- any,
	serializerInCh <-chan any,
	structifier structify.MapStructifier,
) func() error {
	if serializerInCh == nil {
		ch := make(chan any)
		close(ch)
		serializerInCh = ch
	}

	return func() error {
		defer func() {
			close(upsertEntitiesOutCh)
			close(deleteIdsOutCh)
		}()

		for {
			select {
			case message, ok := <-messagesInCh:
				if !ok {
					return nil
				}

				ptr, err := structifier(message.Values)
				if err != nil {
					return errors.Wrapf(err, "can't structify values %#v", message.Values)
				}

				entity, ok := ptr.(database.Entity)
				if !ok {
					return errors.New("ptr does not implement database.Entity")
				}

				runtimeType := message.Values["runtime_type"]
				if runtimeType == nil {
					return errors.Errorf("stream message missing 'runtime_type' key: %v", message.Values)
				}

				if runtimeType == "upsert" {
					select {
					case upsertEntitiesOutCh <- entity:
					case <-ctx.Done():
						return ctx.Err()
					}
				} else if runtimeType == "delete" {
					select {
					case deleteIdsOutCh <- entity.ID():
					case <-ctx.Done():
						return ctx.Err()
					}
				} else {
					return errors.Errorf("invalid runtime type: %s", runtimeType)
				}

				select {
				case <-serializerInCh:
				case <-ctx.Done():
					return ctx.Err()
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// RUOption defines an option for the [RuntimeUpdates.Sync] method.
type RUOption func(*RUOptions)

// RUUpsertFunc defines the type of the callback that can be provided to the [RuntimeUpdates.Sync] method via the [WithRUUpsert] option.
type RUUpsertFunc func(context.Context, database.Entity) error

// RUOptions defines options for the [RuntimeUpdates.Sync] method.
type RUOptions struct {
	allowParallel bool
	upsertFn      RUUpsertFunc
}

// WithAllowParallel allows parallel execution of runtime updates for the same entity type.
func WithAllowParallel() RUOption {
	return func(opts *RUOptions) { opts.allowParallel = true }
}

// WithRUUpsert allows providing a callback that is called for each Redis stream message of type "upsert".
//
// If this option is used, [RuntimeUpdates.Sync] will start a separate xRead goroutine for the corresponding stream,
// thus allowing the provided callback to run in parallel with the main database upsert operations for the same entity
// type. Note that the callback is called for each message of type "upsert", regardless of the entity type, so it is
// the responsibility of that callback to filter the entities if necessary. Also note that the callback must be safe
// for concurrent execution, as it may be called in parallel with different types of entities.
func WithRUUpsert(fn RUUpsertFunc) RUOption {
	return func(opts *RUOptions) { opts.upsertFn = fn }
}
