package icingaredis

import (
	"context"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/database"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/periodic"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"runtime"
	"time"
)

// Client is a wrapper around redis.Client with
// streaming and logging capabilities.
type Client struct {
	*redis.Client

	Options *Options

	logger *logging.Logger
}

// Options define user configurable Redis options.
type Options struct {
	BlockTimeout        time.Duration `yaml:"block_timeout"         default:"1s"`
	HMGetCount          int           `yaml:"hmget_count"           default:"4096"`
	HScanCount          int           `yaml:"hscan_count"           default:"4096"`
	MaxHMGetConnections int           `yaml:"max_hmget_connections" default:"8"`
	Timeout             time.Duration `yaml:"timeout"               default:"30s"`
	XReadCount          int           `yaml:"xread_count"           default:"4096"`
}

// Validate checks constraints in the supplied Redis options and returns an error if they are violated.
func (o *Options) Validate() error {
	if o.BlockTimeout <= 0 {
		return errors.New("block_timeout must be positive")
	}
	if o.HMGetCount < 1 {
		return errors.New("hmget_count must be at least 1")
	}
	if o.HScanCount < 1 {
		return errors.New("hscan_count must be at least 1")
	}
	if o.MaxHMGetConnections < 1 {
		return errors.New("max_hmget_connections must be at least 1")
	}
	if o.Timeout == 0 {
		return errors.New("timeout cannot be 0. Configure a value greater than zero, or use -1 for no timeout")
	}
	if o.XReadCount < 1 {
		return errors.New("xread_count must be at least 1")
	}

	return nil
}

// NewClient returns a new icingaredis.Client wrapper for a pre-existing *redis.Client.
func NewClient(client *redis.Client, logger *logging.Logger, options *Options) *Client {
	return &Client{Client: client, logger: logger, Options: options}
}

// HPair defines Redis hashes field-value pairs.
type HPair struct {
	Field string
	Value string
}

// HYield yields HPair field-value pairs for all fields in the hash stored at key.
func (c *Client) HYield(ctx context.Context, key string) (<-chan HPair, <-chan error) {
	pairs := make(chan HPair, c.Options.HScanCount)

	return pairs, com.WaitAsync(contracts.WaiterFunc(func() error {
		var counter com.Counter
		defer c.log(ctx, key, &counter).Stop()
		defer close(pairs)

		seen := make(map[string]struct{})

		var cursor uint64
		var err error
		var page []string

		for {
			cmd := c.HScan(ctx, key, cursor, "", int64(c.Options.HScanCount))
			page, cursor, err = cmd.Result()

			if err != nil {
				return WrapCmdErr(cmd)
			}

			for i := 0; i < len(page); i += 2 {
				if _, ok := seen[page[i]]; ok {
					// Ignore duplicate returned by HSCAN.
					continue
				}

				seen[page[i]] = struct{}{}

				select {
				case pairs <- HPair{
					Field: page[i],
					Value: page[i+1],
				}:
					counter.Inc()
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			if cursor == 0 {
				break
			}
		}

		return nil
	}))
}

// HMYield yields HPair field-value pairs for the specified fields in the hash stored at key.
func (c *Client) HMYield(ctx context.Context, key string, fields ...string) (<-chan HPair, <-chan error) {
	pairs := make(chan HPair)

	return pairs, com.WaitAsync(contracts.WaiterFunc(func() error {
		var counter com.Counter
		defer c.log(ctx, key, &counter).Stop()

		g, ctx := errgroup.WithContext(ctx)

		defer func() {
			// Wait until the group is done so that we can safely close the pairs channel,
			// because on error, sem.Acquire will return before calling g.Wait(),
			// which can result in goroutines working on a closed channel.
			_ = g.Wait()
			close(pairs)
		}()

		// Use context from group.
		batches := utils.BatchSliceOfStrings(ctx, fields, c.Options.HMGetCount)

		sem := semaphore.NewWeighted(int64(c.Options.MaxHMGetConnections))

		for batch := range batches {
			if err := sem.Acquire(ctx, 1); err != nil {
				return errors.Wrap(err, "can't acquire semaphore")
			}

			batch := batch
			g.Go(func() error {
				defer sem.Release(1)

				cmd := c.HMGet(ctx, key, batch...)
				vals, err := cmd.Result()

				if err != nil {
					return WrapCmdErr(cmd)
				}

				for i, v := range vals {
					if v == nil {
						c.logger.Warnf("HMGET %s: field %#v missing", key, batch[i])
						continue
					}

					select {
					case pairs <- HPair{
						Field: batch[i],
						Value: v.(string),
					}:
						counter.Inc()
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				return nil
			})
		}

		return g.Wait()
	}))
}

// XReadUntilResult (repeatedly) calls XREAD with the specified arguments until a result is returned.
// Each call blocks at most for the duration specified in Options.BlockTimeout until data
// is available before it times out and the next call is made.
// This also means that an already set block timeout is overridden.
func (c *Client) XReadUntilResult(ctx context.Context, a *redis.XReadArgs) ([]redis.XStream, error) {
	a.Block = c.Options.BlockTimeout

	for {
		cmd := c.XRead(ctx, a)
		streams, err := cmd.Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}

			return streams, WrapCmdErr(cmd)
		}

		return streams, nil
	}
}

// YieldAll yields all entities from Redis that belong to the specified SyncSubject.
func (c Client) YieldAll(ctx context.Context, subject *common.SyncSubject) (<-chan database.Entity, <-chan error) {
	key := utils.Key(utils.Name(subject.Entity()), ':')
	if subject.WithChecksum() {
		key = "icinga:checksum:" + key
	} else {
		key = "icinga:" + key
	}

	pairs, errs := c.HYield(ctx, key)
	g, ctx := errgroup.WithContext(ctx)
	// Let errors from HYield cancel the group.
	com.ErrgroupReceive(g, errs)

	desired, errs := CreateEntities(ctx, subject.FactoryForDelta(), pairs, runtime.NumCPU())
	// Let errors from CreateEntities cancel the group.
	com.ErrgroupReceive(g, errs)

	return desired, com.WaitAsync(g)
}

func (c *Client) log(ctx context.Context, key string, counter *com.Counter) periodic.Stopper {
	return periodic.Start(ctx, c.logger.Interval(), func(tick periodic.Tick) {
		// We may never get to progress logging here,
		// as fetching should be completed before the interval expires,
		// but if it does, it is good to have this log message.
		if count := counter.Reset(); count > 0 {
			c.logger.Debugf("Fetched %d items from %s", count, key)
		}
	}, periodic.OnStop(func(tick periodic.Tick) {
		c.logger.Debugf("Finished fetching from %s with %d items in %s", key, counter.Total(), tick.Elapsed)
	}))
}
