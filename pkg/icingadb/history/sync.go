package history

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/icingadb"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1/history"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/structify"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"reflect"
	"sync"
	"time"
)

// Sync specifies the source and destination of a history sync.
type Sync struct {
	db     *icingadb.DB
	redis  *icingaredis.Client
	logger *zap.SugaredLogger
}

// NewSync creates a new Sync.
func NewSync(db *icingadb.DB, redis *icingaredis.Client, logger *zap.SugaredLogger) *Sync {
	return &Sync{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

// Sync synchronizes Redis history streams from s.redis to s.db and deletes the original data on success.
func (s Sync) Sync(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	for key, pipeline := range syncPipelines {
		key := key
		pipeline := pipeline

		s.logger.Debugw("Starting history sync", zap.String("type", key))

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
			i := i
			stage := stage

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
		cmd := s.redis.XRead(ctx, xra)
		streams, err := cmd.Result()

		if err != nil {
			return icingaredis.WrapCmdErr(cmd)
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
	const logInterval = 20 * time.Second

	var count uint64 // Count of synced entries for periodic logging.
	stream := "icinga:history:stream:" + key

	logTicker := time.NewTicker(logInterval)
	defer logTicker.Stop()

	bulks := com.BulkXMessages(ctx, input, s.redis.Options.HScanCount)

	for {
		select {
		case bulk := <-bulks:
			ids := make([]string, len(bulk))
			for i := range bulk {
				ids[i] = bulk[i].ID
			}

			cmd := s.redis.XDel(ctx, stream, ids...)
			if _, err := cmd.Result(); err != nil {
				return icingaredis.WrapCmdErr(cmd)
			}

			count += uint64(len(ids))

		case <-logTicker.C:
			if count > 0 {
				s.logger.Infof("Inserted %d %s history entries in the last %s", count, key, logInterval)
				count = 0
			}

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

func stageFuncForEntity(structPtr interface{}) stageFunc {
	structifier := structify.MakeMapStructifier(reflect.TypeOf(structPtr).Elem(), "json")

	return func(ctx context.Context, s Sync, key string, in <-chan redis.XMessage, out chan<- redis.XMessage) error {
		type State struct {
			Message redis.XMessage // Original event from Redis.
			Pending int            // Number of pending entities. When reaching 0, the message is forwarded to out.
		}

		bufSize := s.db.Options.MaxPlaceholdersPerStatement
		insert := make(chan contracts.Entity, bufSize) // Events sent to the database for insertion.
		inserted := make(chan contracts.Entity)        // Events returned by the database after successful insertion.
		state := make(map[contracts.Entity]*State)     // Shared state between all entities created by one event.
		var stateMu sync.Mutex                         // Synchronizes concurrent access to state.

		g, ctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			defer close(insert)

			for {
				select {
				case e, ok := <-in:
					if !ok {
						return nil
					}

					ptr, err := structifier(e.Values)
					if err != nil {
						return errors.Wrapf(err, "can't structify values %#v", e.Values)
					}

					ue := ptr.(v1.UpserterEntity)

					st := &State{
						Message: e,
						Pending: 1,
					}
					stateMu.Lock()
					state[ue] = st
					stateMu.Unlock()

					select {
					case insert <- ue:
					case <-ctx.Done():
						return ctx.Err()
					}

				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		g.Go(func() error {
			defer close(inserted)

			return s.db.UpsertStreamed(ctx, insert, inserted)
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

				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		return g.Wait()
	}
}

var syncPipelines = map[string][]stageFunc{
	"notification": {
		stageFuncForEntity((*v1.NotificationHistory)(nil)), // notification_history
		stageFuncForEntity((*v1.HistoryNotification)(nil)), // history (depends on notification_history)
	},
	"usernotification": {
		stageFuncForEntity((*v1.UserNotificationHistory)(nil)),
	},
	"state": {
		stageFuncForEntity((*v1.StateHistory)(nil)), // state_history
		stageFuncForEntity((*v1.HistoryState)(nil)), // history (depends on state_history)
	},
	"downtime": {
		stageFuncForEntity((*v1.DowntimeHistory)(nil)), // downtime_history
		stageFuncForEntity((*v1.HistoryDowntime)(nil)), // history (depends on downtime_history)
	},
	"comment": {
		stageFuncForEntity((*v1.CommentHistory)(nil)), // comment_history
		stageFuncForEntity((*v1.HistoryComment)(nil)), // history (depends on comment_history)
	},
	"flapping": {
		stageFuncForEntity((*v1.FlappingHistory)(nil)), // flapping_history
		stageFuncForEntity((*v1.HistoryFlapping)(nil)), // history (depends on flapping_history)
	},
	"acknowledgement": {
		stageFuncForEntity((*v1.AcknowledgementHistory)(nil)), // acknowledgement_history
		stageFuncForEntity((*v1.HistoryAck)(nil)),             // history (depends on acknowledgement_history)
	},
}
