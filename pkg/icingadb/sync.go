package icingadb

import (
	"context"
	"fmt"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"runtime"
	"time"
)

var (
	// Redis concurrency settings.
	count      = 1 << 12
	concurrent = 1 << 3
)

// Sync implements a rendezvous point for Icinga DB and Redis to synchronize their entities.
type Sync struct {
	db     *DB
	redis  *icingaredis.Client
	logger *zap.SugaredLogger
}

func NewSync(db *DB, redis *icingaredis.Client, logger *zap.SugaredLogger) *Sync {
	return &Sync{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

func (s Sync) GetDelta(ctx context.Context, factoryFunc contracts.EntityFactoryFunc) *Delta {
	// Redis key.
	var redisKey string
	// Whether we're syncing an entity that implements contracts.Checksumer.
	var withChecksum bool
	// Value from the factory so that we know what we are synchronizing here.
	v := factoryFunc()
	// Error channel.
	errs := make(chan error, 1)

	if _, ok := v.(contracts.Checksumer); ok {
		withChecksum = true
		redisKey = fmt.Sprintf("icinga:checksum:%s", utils.Key(utils.Name(v), ':'))
	} else {
		redisKey = fmt.Sprintf("icinga:%s", utils.Key(utils.Name(v), ':'))
	}

	desired, err := s.fromRedis(ctx, factoryFunc, redisKey)
	com.PipeError(err, errs)

	actual, err := s.db.YieldAll(ctx, factoryFunc, s.db.BuildSelectStmt(v, v.Fingerprint()))
	com.PipeError(err, errs)

	return NewDelta(ctx, actual, desired, withChecksum, s.logger)
}

// SyncAfterDump waits for a config dump to finish (using the dump parameter) and then starts a sync for the type given
// by factoryFunc using the Sync function.
func (s Sync) SyncAfterDump(ctx context.Context, factoryFunc contracts.EntityFactoryFunc, dump *DumpSignals) error {
	typeName := utils.Name(factoryFunc())
	key := "icinga:" + utils.Key(typeName, ':')

	startTime := time.Now()
	logTicker := time.NewTicker(20 * time.Second)
	loggedWaiting := false
	defer logTicker.Stop()

	for {
		select {
		case <-logTicker.C:
			s.logger.Infow("Waiting for dump done signal",
				zap.String("type", typeName),
				zap.String("key", key),
				zap.Duration("duration", time.Now().Sub(startTime)))
			loggedWaiting = true
		case <-dump.Done(key):
			logFn := s.logger.Debugw
			if loggedWaiting {
				logFn = s.logger.Infow
			}
			logFn("Starting sync",
				zap.String("type", typeName),
				zap.String("key", key),
				zap.Duration("waited", time.Now().Sub(startTime)))
			return s.Sync(ctx, factoryFunc)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Sync synchronizes entities between Icinga DB and Redis created with the specified factory function.
// This function does not respect dump signals. For this, use SyncAfterDump.
func (s Sync) Sync(ctx context.Context, factoryFunc contracts.EntityFactoryFunc) error {
	// Value from the factory so that we know what we are synchronizing here.
	v := factoryFunc()
	// Group for the sync. Whole sync will be cancelled if an error occurs.
	g, ctx := errgroup.WithContext(ctx)

	s.logger.Infof("Syncing %s", utils.Key(utils.Name(v), ' '))

	delta := s.GetDelta(ctx, factoryFunc)
	if err := delta.Wait(); err != nil {
		return err
	}

	// Create
	if len(delta.Create) > 0 {
		var entities <-chan contracts.Entity
		if delta.WithChecksum {
			pairs, errs := s.redis.HMYield(
				ctx,
				fmt.Sprintf("icinga:%s", utils.Key(utils.Name(v), ':')),
				count,
				concurrent,
				delta.Create.Keys()...)
			// Let errors from Redis cancel our group.
			com.ErrgroupReceive(g, errs)

			entitiesWithoutChecksum, errs := icingaredis.CreateEntities(ctx, factoryFunc, pairs, runtime.NumCPU())
			// Let errors from CreateEntities cancel our group.
			com.ErrgroupReceive(g, errs)
			entities, errs = icingaredis.SetChecksums(ctx, entitiesWithoutChecksum, delta.Create, runtime.NumCPU())
			// Let errors from SetChecksums cancel our group.
			com.ErrgroupReceive(g, errs)
		} else {
			entities = delta.Create.Entities(ctx)
		}

		g.Go(func() error {
			return s.db.Create(ctx, entities)
		})
	}

	// Update
	if len(delta.Update) > 0 {
		s.logger.Infof("Updating %d rows of type %s", len(delta.Update), utils.Key(utils.Name(v), ' '))
		pairs, errs := s.redis.HMYield(
			ctx,
			fmt.Sprintf("icinga:%s", utils.Key(utils.Name(v), ':')),
			count,
			concurrent,
			delta.Update.Keys()...)
		// Let errors from Redis cancel our group.
		com.ErrgroupReceive(g, errs)

		entitiesWithoutChecksum, errs := icingaredis.CreateEntities(ctx, factoryFunc, pairs, runtime.NumCPU())
		// Let errors from CreateEntities cancel our group.
		com.ErrgroupReceive(g, errs)
		entities, errs := icingaredis.SetChecksums(ctx, entitiesWithoutChecksum, delta.Update, runtime.NumCPU())
		// Let errors from SetChecksums cancel our group.
		com.ErrgroupReceive(g, errs)

		g.Go(func() error {
			// TODO (el): This is very slow in high latency scenarios.
			// Use strings.Repeat() on the query and create a stmt
			// with a size near the default value of max_allowed_packet.
			return s.db.Update(ctx, entities)
		})
	}

	// Delete
	if len(delta.Delete) > 0 {
		s.logger.Infof("Deleting %d rows of type %s", len(delta.Delete), utils.Key(utils.Name(v), ' '))
		g.Go(func() error {
			return s.db.BulkExec(ctx, s.db.BuildDeleteStmt(v), 1<<15, 1<<3, delta.Delete.IDs())
		})
	}

	return g.Wait()
}

func (s Sync) fromRedis(ctx context.Context, factoryFunc contracts.EntityFactoryFunc, key string) (<-chan contracts.Entity, <-chan error) {
	// Channel for Redis field-value pairs for the specified key and errors.
	pairs, errs := s.redis.HYield(
		ctx, key, count)
	// Group for the Redis sync. Redis sync will be cancelled if an error occurs.
	// Note that we're calling HYield with the original context.
	g, ctx := errgroup.WithContext(ctx)
	// Let errors from HYield cancel our group.
	com.ErrgroupReceive(g, errs)

	desired, errs := icingaredis.CreateEntities(ctx, factoryFunc, pairs, runtime.NumCPU())
	// Let errors from CreateEntities cancel our group.
	com.ErrgroupReceive(g, errs)

	return desired, com.WaitAsync(g)
}
