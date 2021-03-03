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
		redisKey = fmt.Sprintf("icinga:config:%s", utils.Key(utils.Name(v), ':'))
	}

	desired, err := s.fromRedis(ctx, factoryFunc, redisKey)
	com.PipeError(err, errs)

	actual, err := s.db.YieldAll(ctx, factoryFunc, s.db.BuildSelectStmt(v, v.Fingerprint()))
	com.PipeError(err, errs)

	return NewDelta(ctx, actual, desired, withChecksum, s.logger)
}

// Synchronize entities between Icinga DB and Redis created with the specified factory function.
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
	{
		var entities <-chan contracts.Entity
		if delta.WithChecksum {
			pairs, errs := s.redis.HMYield(
				ctx,
				fmt.Sprintf("icinga:config:%s", utils.Key(utils.Name(v), ':')),
				count,
				concurrent,
				utils.SyncMapKeys(delta.Create)...)
			// Let errors from Redis cancel our group.
			com.ErrgroupReceive(g, errs)

			entitiesWithoutChecksum, errs := icingaredis.CreateEntities(ctx, factoryFunc, pairs, runtime.NumCPU())
			// Let errors from CreateEntities cancel our group.
			com.ErrgroupReceive(g, errs)
			entities, errs = icingaredis.SetChecksums(ctx, entitiesWithoutChecksum, delta.Create, runtime.NumCPU())
			// Let errors from SetChecksums cancel our group.
			com.ErrgroupReceive(g, errs)
		} else {
			entities = utils.SyncMapEntities(delta.Create)
		}

		g.Go(func() error {
			return s.db.Create(ctx, entities)
		})
	}

	// Update
	{
		s.logger.Infof("Updating %d rows of type %s", len(utils.SyncMapKeys(delta.Update)), utils.Key(utils.Name(v), ' '))
		pairs, errs := s.redis.HMYield(
			ctx,
			fmt.Sprintf("icinga:config:%s", utils.Key(utils.Name(v), ':')),
			count,
			concurrent,
			utils.SyncMapKeys(delta.Update)...)
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
	{
		s.logger.Infof("Deleting %d rows of type %s", len(utils.SyncMapKeys(delta.Delete)), utils.Key(utils.Name(v), ' '))
		g.Go(func() error {
			return s.db.BulkExec(ctx, s.db.BuildDeleteStmt(v), 1<<15, 1<<3, utils.SyncMapIDs(delta.Delete))
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
