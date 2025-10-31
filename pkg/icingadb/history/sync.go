package history

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/structify"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icingadb/pkg/contracts"
	v1types "github.com/icinga/icingadb/pkg/icingadb/v1"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1/history"
	"github.com/icinga/icingadb/pkg/icingaredis/telemetry"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"reflect"
	"slices"
	"sync"
)

// Sync specifies the source and destination of a history sync.
type Sync struct {
	db     *database.DB
	redis  *redis.Client
	logger *logging.Logger
}

// SyncCallbackConf configures a callback stage given to Sync.Sync.
type SyncCallbackConf struct {
	// Name of this callback, used in [telemetry.Stats].
	Name string
	// KeyStructPtr says which pipeline keys should be mapped to which type, identified by a struct pointer. If
	// a key is missing from the map, it will not be used for the callback.
	KeyStructPtr map[string]any
	// Fn is the actual callback function.
	Fn func(database.Entity) bool
}

// NewSync creates a new Sync.
func NewSync(db *database.DB, redis *redis.Client, logger *logging.Logger) *Sync {
	return &Sync{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

// Sync synchronizes Redis history streams from s.redis to s.db and deletes the original data on success.
//
// It is possible to enable a callback functionality, e.g., for the Icinga Notifications integration. To do so, the
// callbackCfg must be set according to the SyncCallbackConf struct documentation.
func (s Sync) Sync(ctx context.Context, callbackCfg *SyncCallbackConf) error {
	var callbackStageFn stageFunc
	if callbackCfg != nil {
		callbackStageFn = makeSortedCallbackStageFunc(
			ctx,
			s.logger,
			callbackCfg.Name,
			callbackCfg.KeyStructPtr,
			callbackCfg.Fn)
	}

	g, ctx := errgroup.WithContext(ctx)

	for key, pipeline := range syncPipelines {
		s.logger.Debugf("Starting %s history sync", key)

		// The pipeline consists of n+2 stages connected sequentially using n+1 channels of type chan redis.XMessage,
		// where n = len(pipeline), i.e. the number of actual sync stages. So the resulting pipeline looks like this:
		//
		//     readFromRedis()    Reads from redis and sends the history entries to the next stage
		//         ↓ ch[0]
		//     pipeline[0]()      First actual sync stage, receives history items from the previous stage, syncs them
		//                        and once completed, sends them off to the next stage.
		//         ↓ ch[1]
		//        ...             There may be a different number of pipeline stages in between.
		//         ↓ ch[n-1]
		//     pipeline[n-1]()    Last actual sync stage, once it's done, sends the history item to the final stage.
		//         ↓ ch[n]
		//     deleteFromRedis()  After all stages have processed a message successfully, this final stage deletes
		//                        the history entry from the Redis stream as it is now persisted in the database.
		//
		// Each history entry is processed by at most one stage at each time. Each state must forward the entry after
		// it has processed it, even if the stage itself does not do anything with this specific entry. It should only
		// forward the entry after it has completed its own sync so that later stages can rely on previous stages being
		// executed successfully.
		//
		// If a callback exists for this key, it will be appended to the pipeline. Thus, it is executed after every
		// other pipeline action, but before deleteFromRedis.

		var hasCallbackStage bool
		if callbackCfg != nil {
			_, exists := callbackCfg.KeyStructPtr[key]
			hasCallbackStage = exists
		}

		// Shadowed variable to allow appending custom callbacks.
		pipeline := pipeline
		if hasCallbackStage {
			pipeline = append(slices.Clip(pipeline), callbackStageFn)
		}

		ch := make([]chan redis.XMessage, len(pipeline)+1)
		for i := range ch {
			if i == 0 {
				// Make the first channel buffered so that all items of one read iteration fit into the channel.
				// This allows starting the next Redis XREAD right after the previous one has finished.
				ch[i] = make(chan redis.XMessage, s.redis.Options.XReadCount)
			} else {
				ch[i] = make(chan redis.XMessage)
			}
		}

		g.Go(func() error {
			return s.readFromRedis(ctx, key, ch[0])
		})

		for i, stage := range pipeline {
			g.Go(func() error {
				return stage(ctx, s, key, ch[i], ch[i+1])
			})
		}

		g.Go(func() error {
			return s.deleteFromRedis(ctx, key, ch[len(pipeline)])
		})
	}

	return g.Wait()
}

// readFromRedis is the first stage of the history sync pipeline. It reads the history stream from Redis
// and feeds the history entries into the next stage.
func (s Sync) readFromRedis(ctx context.Context, key string, output chan<- redis.XMessage) error {
	defer close(output)

	xra := &redis.XReadArgs{
		Streams: []string{"icinga:history:stream:" + key, "0-0"},
		Count:   int64(s.redis.Options.XReadCount),
	}

	for {
		streams, err := s.redis.XReadUntilResult(ctx, xra)
		if err != nil {
			return errors.Wrap(err, "can't read history")
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				xra.Streams[1] = message.ID

				select {
				case output <- message:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
}

// deleteFromRedis is the last stage of the history sync pipeline. It receives history entries from the second to last
// pipeline stage and then deletes the stream entry from Redis as all pipeline stages successfully processed the entry.
func (s Sync) deleteFromRedis(ctx context.Context, key string, input <-chan redis.XMessage) error {
	var counter com.Counter
	defer periodic.Start(ctx, s.logger.Interval(), func(_ periodic.Tick) {
		if count := counter.Reset(); count > 0 {
			s.logger.Infof("Synced %d %s history items", count, key)
		}
	}).Stop()

	bulks := com.Bulk(ctx, input, s.redis.Options.HScanCount, com.NeverSplit[redis.XMessage])
	stream := "icinga:history:stream:" + key
	for {
		select {
		case bulk, ok := <-bulks:
			if !ok {
				return nil
			}

			ids := make([]string, len(bulk))
			for i := range bulk {
				ids[i] = bulk[i].ID
			}

			cmd := s.redis.XDel(ctx, stream, ids...)
			if _, err := cmd.Result(); err != nil {
				return redis.WrapCmdErr(cmd)
			}

			counter.Add(uint64(len(ids)))
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// stageFunc is a function type that represents a sync pipeline stage. It is called with a context (it should stop
// once that context is canceled), the Sync instance (for access to Redis, SQL database, logging), the key (information
// about which pipeline this function is running in,  i.e. "notification"), an in channel for the stage to read history
// events from and an out channel to forward history entries to after processing them successfully. A stage function
// is supposed to forward each message from in to out, even if the event is not relevant for the current stage. On
// error conditions, the message must not be forwarded to the next stage so that the event is not deleted from Redis
// and can be processed at a later time.
type stageFunc func(ctx context.Context, s Sync, key string, in <-chan redis.XMessage, out chan<- redis.XMessage) error

// writeOneEntityStage creates a stageFunc from a pointer to a struct implementing the v1.UpserterEntity interface.
// For each history event it receives, it parses that event into a new instance of that entity type and writes it to
// the database. It writes exactly one entity to the database for each history event.
func writeOneEntityStage(structPtr any) stageFunc {
	structifier := structify.MakeMapStructifier(
		reflect.TypeOf(structPtr).Elem(),
		"json",
		contracts.SafeInit)

	return writeMultiEntityStage(func(entry redis.XMessage) ([]v1.UpserterEntity, error) {
		ptr, err := structifier(entry.Values)
		if err != nil {
			return nil, errors.Wrapf(err, "can't structify values %#v", entry.Values)
		}
		ptrUpserterEntity, ok := ptr.(v1.UpserterEntity)
		if !ok {
			return nil, errors.New("ptr does not implement UpserterEntity")
		}
		return []v1.UpserterEntity{ptrUpserterEntity}, nil
	})
}

// writeMultiEntityStage creates a stageFunc from a function that takes a history event as an input and returns a
// (potentially empty) slice of v1.UpserterEntity instances that it then inserts into the database.
func writeMultiEntityStage(entryToEntities func(entry redis.XMessage) ([]v1.UpserterEntity, error)) stageFunc {
	return func(ctx context.Context, s Sync, key string, in <-chan redis.XMessage, out chan<- redis.XMessage) error {
		type State struct {
			Message redis.XMessage // Original event from Redis.
			Pending int            // Number of pending entities. When reaching 0, the message is forwarded to out.
		}

		bufSize := s.db.Options.MaxPlaceholdersPerStatement
		insert := make(chan database.Entity, bufSize) // Events sent to the database for insertion.
		inserted := make(chan database.Entity)        // Events returned by the database after successful insertion.
		skipped := make(chan redis.XMessage)          // Events skipping insert/inserted (no entities generated).
		state := make(map[database.Entity]*State)     // Shared state between all entities created by one event.
		var stateMu sync.Mutex                        // Synchronizes concurrent access to state.

		g, ctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			defer close(insert)
			defer close(skipped)

			for {
				select {
				case e, ok := <-in:
					if !ok {
						return nil
					}

					entities, err := entryToEntities(e)
					if err != nil {
						return err
					}

					if len(entities) == 0 {
						skipped <- e
					} else {
						st := &State{
							Message: e,
							Pending: len(entities),
						}

						stateMu.Lock()
						for _, entity := range entities {
							state[entity] = st
						}
						stateMu.Unlock()

						for _, entity := range entities {
							select {
							case insert <- entity:
							case <-ctx.Done():
								return ctx.Err()
							}
						}
					}

				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		g.Go(func() error {
			defer close(inserted)

			return s.db.UpsertStreamed(ctx, insert, database.OnSuccessSendTo[database.Entity](inserted))
		})

		g.Go(func() error {
			defer close(out)

			for {
				select {
				case e, ok := <-inserted:
					if !ok {
						return nil
					}

					stateMu.Lock()
					st := state[e]
					delete(state, e)
					stateMu.Unlock()

					st.Pending--
					if st.Pending == 0 {
						select {
						case out <- st.Message:
						case <-ctx.Done():
							return ctx.Err()
						}
					}

				case m, ok := <-skipped:
					if !ok {
						return nil
					}

					select {
					case out <- m:
					case <-ctx.Done():
						return ctx.Err()
					}

				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		return g.Wait()
	}
}

// userNotificationStage is a specialized stageFunc that populates the user_notification_history table. It is executed
// on the notification history stream and uses the users_notified_ids attribute to create an entry in the
// user_notification_history relation table for each user ID.
func userNotificationStage(ctx context.Context, s Sync, key string, in <-chan redis.XMessage, out chan<- redis.XMessage) error {
	type NotificationHistory struct {
		Id            types.Binary `structify:"id"`
		EnvironmentId types.Binary `structify:"environment_id"`
		EndpointId    types.Binary `structify:"endpoint_id"`
		UserIds       types.String `structify:"users_notified_ids"`
	}

	structifier := structify.MakeMapStructifier(
		reflect.TypeOf((*NotificationHistory)(nil)).Elem(),
		"structify",
		contracts.SafeInit)

	return writeMultiEntityStage(func(entry redis.XMessage) ([]v1.UpserterEntity, error) {
		rawNotificationHistory, err := structifier(entry.Values)
		if err != nil {
			return nil, err
		}
		notificationHistory, ok := rawNotificationHistory.(*NotificationHistory)
		if !ok {
			return nil, errors.New("rawNotificationHistory does not implement NotificationHistory")
		}

		if !notificationHistory.UserIds.Valid {
			return nil, nil
		}

		var users []types.Binary
		err = types.UnmarshalJSON([]byte(notificationHistory.UserIds.String), &users)
		if err != nil {
			return nil, err
		}

		var userNotifications []v1.UpserterEntity

		for _, user := range users {
			userNotifications = append(userNotifications, &v1.UserNotificationHistory{
				EntityWithoutChecksum: v1types.EntityWithoutChecksum{
					IdMeta: v1types.IdMeta{
						Id: utils.Checksum(append(append([]byte(nil), notificationHistory.Id...), user...)),
					},
				},
				EnvironmentMeta: v1types.EnvironmentMeta{
					EnvironmentId: notificationHistory.EnvironmentId,
				},
				NotificationHistoryId: notificationHistory.Id,
				UserId:                user,
			})
		}

		return userNotifications, nil
	})(ctx, s, key, in, out)
}

// countElementStage increments the [Stats.History] counter.
//
// This stageFunc should be called last in a [syncPipeline]. Thus, it is still executed before the final
// Sync.deleteFromRedis call in Sync.Sync. Furthermore, an optional callback function will be appended after this stage,
// resulting in an incremented history state counter for synchronized history, but stalling callback actions.
func countElementStage(ctx context.Context, _ Sync, _ string, in <-chan redis.XMessage, out chan<- redis.XMessage) error {
	defer close(out)

	for {
		select {
		case msg, ok := <-in:
			if !ok {
				return nil
			}

			telemetry.Stats.Get(telemetry.StatHistory).Add(1)
			out <- msg

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// makeSortedCallbackStageFunc creates a new stageFunc calling the callback function after reordering messages.
//
// This stageFunc is designed to be used by multiple channels. The internal sorting logic - realized by a StreamSorter -
// results in all messages to be sorted based on their Redis Stream ID and be ejected to the callback function in this
// order.
//
// The keyStructPtrs map decides what kind of database.Entity type will be used for the input data based on the key.
//
// The callback call is blocking and the message will be forwarded to the out channel after the function has returned.
// Thus, please ensure this function does not block too long.
//
// If the callback function returns false, the message will be retried after an increasing backoff. All subsequent
// messages will wait until this one succeeds.
//
// For each successfully submitted message, the telemetry stat named after this callback is incremented. Thus, a delta
// between [telemetry.StatHistory] and this stat indicates blocking callbacks.
func makeSortedCallbackStageFunc(
	ctx context.Context,
	logger *logging.Logger,
	name string,
	keyStructPtrs map[string]any,
	fn func(database.Entity) bool,
) stageFunc {
	sorterCallbackFn := func(msg redis.XMessage, args any) bool {
		makeEntity := func(key string, values map[string]interface{}) (database.Entity, error) {
			structPtr, ok := keyStructPtrs[key]
			if !ok {
				return nil, fmt.Errorf("key is not part of keyStructPtrs")
			}

			structifier := structify.MakeMapStructifier(
				reflect.TypeOf(structPtr).Elem(),
				"json",
				contracts.SafeInit)
			val, err := structifier(values)
			if err != nil {
				return nil, errors.Wrapf(err, "can't structify values %#v for %q", values, key)
			}

			entity, ok := val.(database.Entity)
			if !ok {
				return nil, fmt.Errorf("structifier returned %T which does not implement database.Entity", val)
			}

			return entity, nil
		}

		key, ok := args.(string)
		if !ok {
			// Shall not happen; set to string some thirty lines below
			panic(fmt.Sprintf("args is of type %T, not string", args))
		}

		entity, err := makeEntity(key, msg.Values)
		if err != nil {
			logger.Errorw("Failed to create database.Entity out of Redis stream message",
				zap.Error(err),
				zap.String("key", key),
				zap.String("id", msg.ID))
			return false
		}

		success := fn(entity)
		if success {
			telemetry.Stats.Get(name).Add(1)
		}
		return success
	}

	sorter := NewStreamSorter(ctx, logger, sorterCallbackFn)

	return func(ctx context.Context, s Sync, key string, in <-chan redis.XMessage, out chan<- redis.XMessage) error {
		defer close(out)

		for {
			select {
			case msg, ok := <-in:
				if !ok {
					return nil
				}

				err := sorter.Submit(msg, key, out)
				if err != nil {
					s.logger.Errorw("Failed to submit Redis stream event to stream sorter", zap.Error(err))
				}

			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

const (
	SyncPipelineAcknowledgement = "acknowledgement"
	SyncPipelineComment         = "comment"
	SyncPipelineDowntime        = "downtime"
	SyncPipelineFlapping        = "flapping"
	SyncPipelineNotification    = "notification"
	SyncPipelineState           = "state"
)

var syncPipelines = map[string][]stageFunc{
	SyncPipelineAcknowledgement: {
		writeOneEntityStage((*v1.AcknowledgementHistory)(nil)), // acknowledgement_history
		writeOneEntityStage((*v1.HistoryAck)(nil)),             // history (depends on acknowledgement_history)
		countElementStage,
	},
	SyncPipelineComment: {
		writeOneEntityStage((*v1.CommentHistory)(nil)), // comment_history
		writeOneEntityStage((*v1.HistoryComment)(nil)), // history (depends on comment_history)
		countElementStage,
	},
	SyncPipelineDowntime: {
		writeOneEntityStage((*v1.DowntimeHistory)(nil)),    // downtime_history
		writeOneEntityStage((*v1.HistoryDowntime)(nil)),    // history (depends on downtime_history)
		writeOneEntityStage((*v1.SlaHistoryDowntime)(nil)), // sla_history_downtime
		countElementStage,
	},
	SyncPipelineFlapping: {
		writeOneEntityStage((*v1.FlappingHistory)(nil)), // flapping_history
		writeOneEntityStage((*v1.HistoryFlapping)(nil)), // history (depends on flapping_history)
		countElementStage,
	},
	SyncPipelineNotification: {
		writeOneEntityStage((*v1.NotificationHistory)(nil)), // notification_history
		userNotificationStage,                               // user_notification_history (depends on notification_history)
		writeOneEntityStage((*v1.HistoryNotification)(nil)), // history (depends on notification_history)
		countElementStage,
	},
	SyncPipelineState: {
		writeOneEntityStage((*v1.StateHistory)(nil)),   // state_history
		writeOneEntityStage((*v1.HistoryState)(nil)),   // history (depends on state_history)
		writeMultiEntityStage(stateHistoryToSlaEntity), // sla_history_state
		countElementStage,
	},
}
