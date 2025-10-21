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
	"time"
)

// Sync specifies the source and destination of a history sync.
type Sync struct {
	db     *database.DB
	redis  *redis.Client
	logger *logging.Logger
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
// optional callbackFn and callbackKeyStructPtr must be set. Both must either be nil or not nil. If set, the additional
// callbackName must also be set, to be used in [telemetry.Stats].
//
// The callbackKeyStructPtr says which pipeline keys should be mapped to which type, identified by a struct pointer. If
// a key is missing from the map, it will not be used for the callback. The callbackFn function shall not block.
func (s Sync) Sync(
	ctx context.Context,
	callbackName string,
	callbackKeyStructPtr map[string]any,
	callbackFn func(database.Entity) bool,
) error {
	if (callbackKeyStructPtr == nil) != (callbackFn == nil) {
		return fmt.Errorf("either both callbackKeyStructPtr and callbackFn must be nil or none")
	}
	if (callbackKeyStructPtr != nil) && (callbackName == "") {
		return fmt.Errorf("if callbackKeyStructPtr and callbackFn are set, a callbackName is required")
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
		if callbackKeyStructPtr != nil {
			_, exists := callbackKeyStructPtr[key]
			hasCallbackStage = exists
		}

		// Shadowed variable to allow appending custom callbacks.
		pipeline := pipeline
		if hasCallbackStage {
			pipeline = append(slices.Clip(pipeline), makeCallbackStageFunc(callbackName, callbackKeyStructPtr, callbackFn))
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

// makeCallbackStageFunc creates a new stageFunc calling the given callback function for each message.
//
// The keyStructPtrs map decides what kind of database.Entity type will be used for the input data based on the key.
//
// The callback call is blocking and the message will be forwarded to the out channel after the function has returned.
// Thus, please ensure this function does not block too long.
//
// If the callback function returns false, the stageFunc switches to a backlog mode, retrying the failed messages and
// every subsequent message until there are no messages left. Only after a message was successfully handled by the
// callback method, it will be forwarded to the out channel. Thus, this stage might "block" or "hold back" certain
// messages during unhappy callback times.
//
// For each successfully submitted message, the telemetry stat named after this callback is incremented. Thus, a delta
// between [telemetry.StatHistory] and this stat indicates blocking callbacks.
func makeCallbackStageFunc(
	name string,
	keyStructPtrs map[string]any,
	fn func(database.Entity) bool,
) stageFunc {
	return func(ctx context.Context, s Sync, key string, in <-chan redis.XMessage, out chan<- redis.XMessage) error {
		defer close(out)

		structPtr, ok := keyStructPtrs[key]
		if !ok {
			return fmt.Errorf("can't lookup struct pointer for key %q", key)
		}

		structifier := structify.MakeMapStructifier(
			reflect.TypeOf(structPtr).Elem(),
			"json",
			contracts.SafeInit)

		makeEntity := func(values map[string]interface{}) (database.Entity, error) {
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

		backlogLastId := ""
		backlogMsgCounter := 0

		const backlogTimerMinInterval, backlogTimerMaxInterval = time.Millisecond, time.Minute
		backlogTimerInterval := backlogTimerMinInterval
		backlogTimer := time.NewTimer(backlogTimerInterval)
		_ = backlogTimer.Stop()

		for {
			select {
			case msg, ok := <-in:
				if !ok {
					return nil
				}

				// Only submit the entity directly if there is no backlog.
				// The second check covers a potential corner case if the XRANGE below races this stream.
				if backlogLastId != "" && backlogLastId != msg.ID {
					continue
				}

				entity, err := makeEntity(msg.Values)
				if err != nil {
					return err
				}

				if fn(entity) {
					out <- msg
					telemetry.Stats.Get(name).Add(1)
					backlogLastId = ""
				} else {
					backlogLastId = msg.ID
					backlogMsgCounter = 0
					backlogTimerInterval = backlogTimerMinInterval
					_ = backlogTimer.Reset(backlogTimerInterval)
					s.logger.Warnw("Failed to submit entity to callback, entering into backlog",
						zap.String("key", key),
						zap.String("id", backlogLastId))
				}

			case <-backlogTimer.C:
				if backlogLastId == "" { // Should never happen.
					return fmt.Errorf("backlog timer logic for %q was called while backlogLastId was empty", key)
				}

				logger := s.logger.With(
					zap.String("key", key),
					zap.String("last-id", backlogLastId))

				logger.Debug("Trying to advance backlog of callback elements")

				xrangeCmd := s.redis.XRangeN(ctx, "icinga:history:stream:"+key, backlogLastId, "+", 2)
				msgs, err := xrangeCmd.Result()
				if err != nil {
					return errors.Wrapf(err, "XRANGE %q to %q on stream %q failed", backlogLastId, "+", key)
				}

				if len(msgs) < 1 || len(msgs) > 2 {
					return fmt.Errorf("XRANGE %q to %q on stream %q returned %d messages, not 1 or 2",
						backlogLastId, "+", key, len(msgs))
				}

				msg := msgs[0]
				entity, err := makeEntity(msg.Values)
				if err != nil {
					return errors.Wrapf(err, "can't structify backlog value %q for %q", backlogLastId, key)
				}

				if fn(entity) {
					out <- msg
					backlogMsgCounter++
					telemetry.Stats.Get(name).Add(1)

					if len(msgs) == 1 {
						backlogLastId = ""
						logger.Infow("Finished rolling back backlog of callback elements", zap.Int("delay", backlogMsgCounter))
					} else {
						backlogLastId = msgs[1].ID
						backlogTimerInterval = backlogTimerMinInterval
						_ = backlogTimer.Reset(backlogTimerInterval)
						logger.Debugw("Advanced backlog",
							zap.String("new-last-id", backlogLastId),
							zap.Duration("delay", backlogTimerInterval))
					}
				} else {
					backlogTimerInterval = min(backlogTimerMaxInterval, backlogTimerInterval*2)
					_ = backlogTimer.Reset(backlogTimerInterval)
					logger.Warnw("Failed to roll back callback elements", zap.Duration("delay", backlogTimerInterval))
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
