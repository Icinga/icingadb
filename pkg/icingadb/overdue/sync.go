package overdue

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingadb/v1/overdue"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/icingaredis/telemetry"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/periodic"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Sync specifies the source and destination of an overdue sync.
type Sync struct {
	db     *icingadb.DB
	redis  *icingaredis.Client
	logger *logging.Logger
}

// NewSync creates a new Sync.
func NewSync(db *icingadb.DB, redis *icingaredis.Client, logger *logging.Logger) *Sync {
	return &Sync{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

// factory abstracts overdue.NewHostState and overdue.NewServiceState.
type factory = func(id string, overdue bool) (contracts.Entity, error)

// Sync synchronizes Redis overdue sets from s.redis to s.db.
func (s Sync) Sync(ctx context.Context) error {
	{
		g, ctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			return s.initSync(ctx, "host")
		})

		g.Go(func() error {
			return s.initSync(ctx, "service")
		})

		if err := g.Wait(); err != nil {
			return errors.Wrap(err, "can't sync overdue indicators")
		}
	}

	g, ctx := errgroup.WithContext(ctx)

	var hostCounter com.Counter
	defer s.log(ctx, "host", &hostCounter).Stop()

	var serviceCounter com.Counter
	defer s.log(ctx, "service", &serviceCounter).Stop()

	g.Go(func() error {
		return s.sync(ctx, "host", overdue.NewHostState, &hostCounter)
	})

	g.Go(func() error {
		return s.sync(ctx, "service", overdue.NewServiceState, &serviceCounter)
	})

	return g.Wait()
}

// initSync initializes icingadb:overdue:objectType from the database.
func (s Sync) initSync(ctx context.Context, objectType string) error {
	s.logger.Debugf("Refreshing already synced %s overdue indicators", objectType)
	start := time.Now()

	var rows []v1.IdMeta
	query := fmt.Sprintf("SELECT id FROM %s_state WHERE is_overdue='y'", objectType)

	if err := s.db.SelectContext(ctx, &rows, query); err != nil {
		return internal.CantPerformQuery(err, query)
	}

	_, err := s.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		key := "icingadb:overdue:" + objectType
		pipe.Del(ctx, key)

		var ids []interface{}
		for _, row := range rows {
			ids = append(ids, row.Id.String())
			if len(ids) == 100 {
				pipe.SAdd(ctx, key, ids...)
				ids = nil
			}
		}

		if len(ids) > 0 {
			pipe.SAdd(ctx, key, ids...)
		}

		return nil
	})

	if err == nil {
		s.logger.Debugf(
			"Refreshing %d already synced %s overdue indicators took %s",
			len(rows), objectType, time.Since(start),
		)
	} else {
		err = errors.Wrap(err, "can't execute Redis pipeline")
	}

	return err
}

// log periodically logs sync's workload.
func (s Sync) log(ctx context.Context, objectType string, counter *com.Counter) periodic.Stopper {
	return periodic.Start(ctx, s.logger.Interval(), func(_ periodic.Tick) {
		if count := counter.Reset(); count > 0 {
			s.logger.Infof("Synced %d %s overdue indicators", count, objectType)
		}
	})
}

//go:embed get_overdues.lua
var getOverduesLua string

var luaGetOverdues = redis.NewScript(strings.TrimSpace(
	regexp.MustCompile(`(?m)^--.*?$`).ReplaceAllString(getOverduesLua, ""),
))

// sync synchronizes Redis overdue sets from s.redis to s.db for objectType.
func (s Sync) sync(ctx context.Context, objectType string, factory factory, counter *com.Counter) error {
	s.logger.Debugf("Syncing %s overdue indicators", objectType)

	keys := [3]string{"icinga:nextupdate:" + objectType, "icingadb:overdue:" + objectType, ""}
	if rand, err := uuid.NewRandom(); err == nil {
		keys[2] = rand.String()
	} else {
		return errors.Wrap(err, "can't create random UUID")
	}

	const period = 2 * time.Second
	periodically := time.NewTicker(period)
	defer periodically.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-periodically.C:
			overdues, err := luaGetOverdues.Run(
				ctx, s.redis, keys[:], strconv.FormatInt(time.Now().Unix(), 10),
			).Result()
			if err != nil {
				return errors.Wrap(err, "can't execute Redis script")
			}

			root := overdues.([]interface{})
			g, ctx := errgroup.WithContext(ctx)

			g.Go(func() error {
				return s.updateOverdue(ctx, objectType, factory, counter, root[0].([]interface{}), true)
			})

			g.Go(func() error {
				return s.updateOverdue(ctx, objectType, factory, counter, root[1].([]interface{}), false)
			})

			if err := g.Wait(); err != nil {
				return errors.Wrap(err, "can't update overdue indicators")
			}

			// For the case that syncing has taken some time, delay the next sync.
			periodically.Reset(period)

			select {
			case <-periodically.C: // Clean up periodically.C after reset...
			default: // ... unless it's already clean.
			}
		}
	}
}

// updateOverdue sets objectType_state#is_overdue for ids to overdue
// and updates icingadb:overdue:objectType respectively.
func (s Sync) updateOverdue(
	ctx context.Context, objectType string, factory factory, counter *com.Counter, ids []interface{}, overdue bool,
) error {
	if len(ids) < 1 {
		return nil
	}

	if err := s.updateDb(ctx, factory, ids, overdue); err != nil {
		return errors.Wrap(err, "can't update overdue indicators")
	}

	counter.Add(uint64(len(ids)))
	telemetry.Stats.Overdue.Add(uint64(len(ids)))

	var op func(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	if overdue {
		op = s.redis.SAdd
	} else {
		op = s.redis.SRem
	}

	_, err := op(ctx, "icingadb:overdue:"+objectType, ids...).Result()
	return err
}

// updateDb sets objectType_state#is_overdue for ids to overdue.
func (s Sync) updateDb(ctx context.Context, factory factory, ids []interface{}, overdue bool) error {
	g, ctx := errgroup.WithContext(ctx)
	ch := make(chan contracts.Entity, 1<<10)

	g.Go(func() error {
		defer close(ch)

		for _, id := range ids {
			e, err := factory(id.(string), overdue)
			if err != nil {
				return errors.Wrap(err, "can't create entity")
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- e:
			}
		}

		return nil
	})

	g.Go(func() error {
		return s.db.UpdateStreamed(ctx, ch)
	})

	return g.Wait()
}
