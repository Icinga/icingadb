package history

import (
	"context"
	"github.com/go-redis/redis/v8"
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

// insertedMessage represents a just inserted row.
type insertedMessage struct {
	// redisId specifies the origin Redis message.
	redisId string
	// structType represents the table the row was inserted into.
	structType reflect.Type
}

const bulkSize = 1 << 14

// Sync synchronizes Redis history streams from s.redis to s.db and deletes the original data on success.
func (s Sync) Sync(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, hs := range historyStreams {
		var redis2structs []chan<- redis.XMessage
		insertedMessages := make(chan insertedMessage, bulkSize)

		// messageProgress are the tables (represented by struct types)
		// with successfully inserted rows by Redis message ID.
		messageProgress := map[string]map[reflect.Type]struct{}{}
		messageProgressMtx := &sync.Mutex{}

		stream := "icinga:history:stream:" + hs.kind
		s.logger.Infof("Syncing %s history", hs.kind)

		for _, structifier := range hs.structifiers {
			redis2struct := make(chan redis.XMessage, bulkSize)
			struct2db := make(chan contracts.Entity, bulkSize)
			succeeded := make(chan contracts.Entity, bulkSize)

			// rowIds are IDs of to be synced Redis messages by database row.
			rowIds := map[contracts.Entity]string{}
			rowIdsMtx := &sync.Mutex{}

			redis2structs = append(redis2structs, redis2struct)

			g.Go(structifyStream(ctx, structifier, redis2struct, struct2db, rowIds, rowIdsMtx))
			g.Go(fwdSucceeded(ctx, insertedMessages, succeeded, rowIds, rowIdsMtx))

			// Upserts from struct2db.
			g.Go(func() error {
				defer close(succeeded)
				return s.db.UpsertStreamed(ctx, struct2db, succeeded)
			})
		}

		g.Go(s.xRead(ctx, redis2structs, stream))
		g.Go(s.cleanup(ctx, hs, insertedMessages, messageProgress, messageProgressMtx, stream))
	}

	return g.Wait()
}

// xRead reads from the Redis stream and broadcasts the data to redis2structs.
func (s Sync) xRead(ctx context.Context, redis2structs []chan<- redis.XMessage, stream string) func() error {
	return func() error {
		defer func() {
			for _, r2s := range redis2structs {
				close(r2s)
			}
		}()

		xra := &redis.XReadArgs{
			Streams: []string{stream, "0-0"},
			Count:   bulkSize,
			Block:   10 * time.Second,
		}

		for {
			cmd := s.redis.XRead(ctx, xra)
			streams, err := cmd.Result()

			if err != nil && err != redis.Nil {
				return icingaredis.WrapCmdErr(cmd)
			}

			for _, stream := range streams {
				for _, message := range stream.Messages {
					xra.Streams[1] = message.ID

					for _, r2s := range redis2structs {
						select {
						case <-ctx.Done():
							return ctx.Err()
						case r2s <- message:
						}
					}
				}
			}
		}
	}
}

// structifyStream structifies from redis2struct to struct2db.
func structifyStream(
	ctx context.Context, structifier structify.MapStructifier, redis2struct <-chan redis.XMessage,
	struct2db chan<- contracts.Entity, rowIds map[contracts.Entity]string, rowIdsMtx *sync.Mutex,
) func() error {
	return func() error {
		defer close(struct2db)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case message, ok := <-redis2struct:
				if !ok {
					return nil
				}

				ptr, err := structifier(message.Values)
				if err != nil {
					return errors.Wrapf(err, "can't structify values %#v", message.Values)
				}

				ue := ptr.(v1.UpserterEntity)

				rowIdsMtx.Lock()
				rowIds[ue] = message.ID
				rowIdsMtx.Unlock()

				select {
				case <-ctx.Done():
					return ctx.Err()
				case struct2db <- ue:
				}
			}
		}
	}
}

// fwdSucceeded informs insertedMessages about successfully inserted rows according to succeeded.
func fwdSucceeded(
	ctx context.Context, insertedMessages chan<- insertedMessage, succeeded <-chan contracts.Entity,
	rowIds map[contracts.Entity]string, rowIdsMtx *sync.Mutex,
) func() error {
	return func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case row, ok := <-succeeded:
				if !ok {
					return nil
				}

				rowIdsMtx.Lock()

				id, ok := rowIds[row]
				if ok {
					delete(rowIds, row)
				}

				rowIdsMtx.Unlock()

				if ok {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case insertedMessages <- insertedMessage{id, reflect.TypeOf(row).Elem()}:
					}
				}
			}
		}
	}
}

// cleanup collects completely inserted messages from insertedMessages and deletes them from Redis.
func (s Sync) cleanup(
	ctx context.Context, hs historyStream, insertedMessages <-chan insertedMessage,
	messageProgress map[string]map[reflect.Type]struct{}, messageProgressMtx *sync.Mutex, stream string,
) func() error {
	return func() error {
		var ids []string
		var count uint64
		var timeout <-chan time.Time

		const period = 20 * time.Second
		periodically := time.NewTicker(period)
		defer periodically.Stop()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-periodically.C:
				if count > 0 {
					s.logger.Infof("Inserted %d %s history entries in the last %s", count, hs.kind, period)
					count = 0
				}
			case msg := <-insertedMessages:
				messageProgressMtx.Lock()

				mp, ok := messageProgress[msg.redisId]
				if !ok {
					mp = map[reflect.Type]struct{}{}
					messageProgress[msg.redisId] = mp
				}

				mp[msg.structType] = struct{}{}

				if ok = len(mp) == len(hs.structifiers); ok {
					delete(messageProgress, msg.redisId)
				}

				messageProgressMtx.Unlock()

				if ok {
					ids = append(ids, msg.redisId)
					count++

					switch len(ids) {
					case 1:
						timeout = time.After(time.Second / 4)
					case bulkSize:
						cmd := s.redis.XDel(ctx, stream, ids...)
						if _, err := cmd.Result(); err != nil {
							return icingaredis.WrapCmdErr(cmd)
						}

						ids = nil
						timeout = nil
					}
				}
			case <-timeout:
				cmd := s.redis.XDel(ctx, stream, ids...)
				if _, err := cmd.Result(); err != nil {
					return icingaredis.WrapCmdErr(cmd)
				}

				ids = nil
				timeout = nil
			}
		}
	}
}

// historyStream represents a Redis history stream.
type historyStream struct {
	// kind specifies the stream's purpose.
	kind string
	// structifiers lists the factories of the model structs the stream data shall be copied to.
	structifiers []structify.MapStructifier
}

// historyStreams contains all Redis history streams to sync.
var historyStreams = func() []historyStream {
	var streams []historyStream
	for _, rhs := range []struct {
		kind       string
		structPtrs []v1.UpserterEntity
	}{
		{"notification", []v1.UpserterEntity{(*v1.NotificationHistory)(nil), (*v1.HistoryNotification)(nil)}},
		{"usernotification", []v1.UpserterEntity{(*v1.UserNotificationHistory)(nil)}},
		{"state", []v1.UpserterEntity{(*v1.StateHistory)(nil), (*v1.HistoryState)(nil)}},
		{"downtime", []v1.UpserterEntity{(*v1.DowntimeHistory)(nil), (*v1.HistoryDowntime)(nil)}},
		{"comment", []v1.UpserterEntity{(*v1.CommentHistory)(nil), (*v1.HistoryComment)(nil)}},
		{"flapping", []v1.UpserterEntity{(*v1.FlappingHistory)(nil), (*v1.HistoryFlapping)(nil)}},
		{"acknowledgement", []v1.UpserterEntity{(*v1.AcknowledgementHistory)(nil), (*v1.HistoryAck)(nil)}},
	} {
		var structifiers []structify.MapStructifier
		for _, structPtr := range rhs.structPtrs {
			structifiers = append(structifiers, structify.MakeMapStructifier(reflect.TypeOf(structPtr).Elem(), "json"))
		}

		streams = append(streams, historyStream{rhs.kind, structifiers})
	}

	return streams
}()
