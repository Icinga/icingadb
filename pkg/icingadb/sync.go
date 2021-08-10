package icingadb

import (
	"context"
	"fmt"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"runtime"
	"time"
)

// Sync implements a rendezvous point for Icinga DB and Redis to synchronize their entities.
type Sync struct {
	db     *DB
	redis  *icingaredis.Client
	logger *zap.SugaredLogger
}

// NewSync returns a new Sync.
func NewSync(db *DB, redis *icingaredis.Client, logger *zap.SugaredLogger) *Sync {
	return &Sync{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

// SyncAfterDump waits for a config dump to finish (using the dump parameter) and then starts a sync for the given
// sync subject using the Sync function.
func (s Sync) SyncAfterDump(ctx context.Context, subject *common.SyncSubject, dump *DumpSignals) error {
	typeName := utils.Name(subject.Entity())
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
				zap.Duration("duration", time.Since(startTime)))
			loggedWaiting = true
		case <-dump.Done(key):
			logFn := s.logger.Debugw
			if loggedWaiting {
				logFn = s.logger.Infow
			}
			logFn("Starting sync",
				zap.String("type", typeName),
				zap.String("key", key),
				zap.Duration("waited", time.Since(startTime)))
			return s.Sync(ctx, subject)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Sync synchronizes entities between Icinga DB and Redis created with the specified sync subject.
// This function does not respect dump signals. For this, use SyncAfterDump.
func (s Sync) Sync(ctx context.Context, subject *common.SyncSubject) error {
	s.logger.Infof("Syncing %s", utils.Key(utils.Name(subject.Entity()), ' '))

	g, ctx := errgroup.WithContext(ctx)

	desired, redisErrs := s.redis.YieldAll(ctx, subject)
	// Let errors from Redis cancel our group.
	com.ErrgroupReceive(g, redisErrs)

	e, ok := v1.EnvironmentFromContext(ctx)
	if !ok {
		return errors.New("can't get environment from context")
	}

	actual, dbErrs := s.db.YieldAll(
		ctx, subject.Factory(), s.db.BuildSelectStmt(NewScopedEntity(subject.Entity(), e.Meta()), subject.Entity().Fingerprint()), e.Meta())
	// Let errors from DB cancel our group.
	com.ErrgroupReceive(g, dbErrs)

	g.Go(func() error {
		return s.ApplyDelta(ctx, NewDelta(ctx, actual, desired, subject, s.logger))
	})

	return g.Wait()
}

// ApplyDelta applies all changes from Delta to the database.
func (s Sync) ApplyDelta(ctx context.Context, delta *Delta) error {
	if err := delta.Wait(); err != nil {
		return errors.Wrap(err, "can't calculate delta")
	}

	g, ctx := errgroup.WithContext(ctx)

	// Create
	if len(delta.Create) > 0 {
		var entities <-chan contracts.Entity
		if delta.Subject.WithChecksum() {
			pairs, errs := s.redis.HMYield(
				ctx,
				fmt.Sprintf("icinga:%s", utils.Key(utils.Name(delta.Subject.Entity()), ':')),
				delta.Create.Keys()...)
			// Let errors from Redis cancel our group.
			com.ErrgroupReceive(g, errs)

			entitiesWithoutChecksum, errs := icingaredis.CreateEntities(ctx, delta.Subject.Factory(), pairs, runtime.NumCPU())
			// Let errors from CreateEntities cancel our group.
			com.ErrgroupReceive(g, errs)
			entities, errs = icingaredis.SetChecksums(ctx, entitiesWithoutChecksum, delta.Create, runtime.NumCPU())
			// Let errors from SetChecksums cancel our group.
			com.ErrgroupReceive(g, errs)
		} else {
			entities = delta.Create.Entities(ctx)
		}

		g.Go(func() error {
			return s.db.CreateStreamed(ctx, entities)
		})
	}

	// Update
	if len(delta.Update) > 0 {
		s.logger.Infof("Updating %d rows of type %s", len(delta.Update), utils.Key(utils.Name(delta.Subject.Entity()), ' '))
		pairs, errs := s.redis.HMYield(
			ctx,
			fmt.Sprintf("icinga:%s", utils.Key(utils.Name(delta.Subject.Entity()), ':')),
			delta.Update.Keys()...)
		// Let errors from Redis cancel our group.
		com.ErrgroupReceive(g, errs)

		entitiesWithoutChecksum, errs := icingaredis.CreateEntities(ctx, delta.Subject.Factory(), pairs, runtime.NumCPU())
		// Let errors from CreateEntities cancel our group.
		com.ErrgroupReceive(g, errs)
		entities, errs := icingaredis.SetChecksums(ctx, entitiesWithoutChecksum, delta.Update, runtime.NumCPU())
		// Let errors from SetChecksums cancel our group.
		com.ErrgroupReceive(g, errs)

		g.Go(func() error {
			// Using upsert here on purpose as this is the fastest way to do bulk updates.
			// However, there is a risk that errors in the sync implementation could silently insert new rows.
			return s.db.UpsertStreamed(ctx, entities, nil)
		})
	}

	// Delete
	if len(delta.Delete) > 0 {
		s.logger.Infof("Deleting %d rows of type %s", len(delta.Delete), utils.Key(utils.Name(delta.Subject.Entity()), ' '))
		g.Go(func() error {
			return s.db.Delete(ctx, delta.Subject.Entity(), delta.Delete.IDs())
		})
	}

	return g.Wait()
}
