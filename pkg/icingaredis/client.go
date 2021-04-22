package icingaredis

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"time"
)

// Client is a wrapper around redis.Client with
// streaming and logging capabilities.
type Client struct {
	*redis.Client

	logger *zap.SugaredLogger
}

// NewClient returns a new icingaredis.Client wrapper for a pre-existing *redis.Client.
func NewClient(client *redis.Client, logger *zap.SugaredLogger) *Client {
	return &Client{Client: client, logger: logger}
}

// HPair defines Redis hashes field-value pairs.
type HPair struct {
	Field string
	Value string
}

// HYield yields HPair field-value pairs for all fields in the hash stored at key.
func (c *Client) HYield(ctx context.Context, key string, count int) (<-chan HPair, <-chan error) {
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
			page, cursor, err = c.HScan(
				ctx, key, cursor, "", int64(count)).Result()
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
func (c *Client) HMYield(ctx context.Context, key string, count int, concurrent int, fields ...string) (<-chan HPair, <-chan error) {
	pairs := make(chan HPair)
	g, ctx := errgroup.WithContext(ctx)
	// Use context from group.
	batches := utils.BatchSliceOfStrings(ctx, fields, count)

	g.Go(func() error {
		defer close(pairs)

		sem := semaphore.NewWeighted(int64(concurrent))

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
