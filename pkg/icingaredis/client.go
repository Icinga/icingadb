package icingaredis

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"runtime"
	"time"
)

// Client is a wrapper around redis.Client with
// streaming and logging capabilities.
type Client struct {
	*redis.Client

	logger  *zap.SugaredLogger
	options *Options
}

type Options struct {
	Timeout             time.Duration `yaml:"timeout"               default:"30s"`
	MaxHMGetConnections int           `yaml:"max_hmget_connections" default:"4096"`
	HMGetCount          int           `yaml:"hmget_count"           default:"4096"`
	HScanCount          int           `yaml:"hscan_count"           default:"4096"`
}

// NewClient returns a new icingaredis.Client wrapper for a pre-existing *redis.Client.
func NewClient(client *redis.Client, logger *zap.SugaredLogger, options *Options) *Client {
	return &Client{Client: client, logger: logger, options: options}
}

// HPair defines Redis hashes field-value pairs.
type HPair struct {
	Field string
	Value string
}

// HYield yields HPair field-value pairs for all fields in the hash stored at key.
func (c *Client) HYield(ctx context.Context, key string) (<-chan HPair, <-chan error) {
	pairs := make(chan HPair)
	g, ctx := errgroup.WithContext(ctx)

	c.logger.Infof("Syncing %s", key)

	g.Go(func() error {
		var cnt com.Counter

		defer close(pairs)
		defer utils.Timed(time.Now(), func(elapsed time.Duration) {
			c.logger.Infof("Fetched %d elements of %s in %s", cnt.Val(), key, elapsed)
		})

		var cursor uint64
		var err error
		var page []string

		g, ctx := errgroup.WithContext(ctx)

		for {
			page, cursor, err = c.HScan(ctx, key, cursor, "", int64(c.options.HScanCount)).Result()
			if err != nil {
				return err
			}

			g.Go(func(page []string) func() error {
				return func() error {
					for i := 0; i < len(page); i += 2 {
						select {
						case pairs <- HPair{
							Field: page[i],
							Value: page[i+1],
						}:
							cnt.Inc()
						case <-ctx.Done():
							return ctx.Err()
						}
					}

					return nil
				}
			}(page))

			if cursor == 0 {
				break
			}
		}

		return g.Wait()
	})

	return pairs, com.WaitAsync(g)
}

// HMYield yields HPair field-value pairs for the specified fields in the hash stored at key.
func (c *Client) HMYield(ctx context.Context, key string, fields ...string) (<-chan HPair, <-chan error) {
	pairs := make(chan HPair)
	g, ctx := errgroup.WithContext(ctx)
	// Use context from group.
	batches := utils.BatchSliceOfStrings(ctx, fields, c.options.HMGetCount)

	g.Go(func() error {
		defer close(pairs)

		sem := semaphore.NewWeighted(int64(c.options.MaxHMGetConnections))

		g, ctx := errgroup.WithContext(ctx)

		for batch := range batches {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}

			g.Go(func(batch []string) func() error {
				return func() error {
					defer sem.Release(1)

					vals, err := c.HMGet(ctx, key, batch...).Result()
					if err != nil {
						return err
					}

					g.Go(func() error {
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
							case <-ctx.Done():
								return ctx.Err()
							}
						}

						return nil
					})

					return nil
				}
			}(batch))
		}

		return g.Wait()
	})

	return pairs, com.WaitAsync(g)
}

// StreamLastId fetches the last message of a stream and returns its ID.
func (c *Client) StreamLastId(ctx context.Context, stream string) (string, error) {
	lastId := "0-0"

	messages, err := c.XRevRangeN(ctx, stream, "+", "-", 1).Result()
	if err != nil {
		return "", err
	}

	for _, message := range messages {
		lastId = message.ID
	}

	return lastId, nil
}

// YieldAll yields all entities from Redis that belong to the specified SyncSubject.
func (c Client) YieldAll(ctx context.Context, subject *common.SyncSubject) (<-chan contracts.Entity, <-chan error) {
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

	desired, errs := CreateEntities(ctx, subject.Factory(), pairs, runtime.NumCPU())
	// Let errors from CreateEntities cancel the group.
	com.ErrgroupReceive(g, errs)

	return desired, com.WaitAsync(g)
}
