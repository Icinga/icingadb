package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/icinga/icingadb/pkg/backoff"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/periodic"
	"github.com/icinga/icingadb/pkg/retry"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"net"
	"time"
)

// Client is a wrapper around redis.Client with
// streaming and logging capabilities.
type Client struct {
	*redis.Client

	Options *Options

	logger *logging.Logger
}

// NewClient returns a new Client wrapper for a pre-existing redis.Client.
func NewClient(client *redis.Client, logger *logging.Logger, options *Options) *Client {
	return &Client{Client: client, logger: logger, Options: options}
}

// NewClientFromConfig returns a new Client from Config.
func NewClientFromConfig(c *Config, logger *logging.Logger) (*Client, error) {
	tlsConfig, err := c.TlsOptions.MakeConfig(c.Host)
	if err != nil {
		return nil, err
	}

	var dialer ctxDialerFunc
	dl := &net.Dialer{Timeout: 15 * time.Second}

	if tlsConfig == nil {
		dialer = dl.DialContext
	} else {
		dialer = (&tls.Dialer{NetDialer: dl, Config: tlsConfig}).DialContext
	}

	options := &redis.Options{
		Dialer:      dialWithLogging(dialer, logger),
		Password:    c.Password,
		DB:          0, // Use default DB,
		ReadTimeout: c.Options.Timeout,
		TLSConfig:   tlsConfig,
	}

	if utils.IsUnixAddr(c.Host) {
		options.Network = "unix"
		options.Addr = c.Host
	} else {
		port := c.Port
		if port == 0 {
			port = 6379
		}
		options.Network = "tcp"
		options.Addr = net.JoinHostPort(c.Host, fmt.Sprint(port))
	}

	client := redis.NewClient(options)
	options = client.Options()
	options.PoolSize = utils.MaxInt(32, options.PoolSize)
	options.MaxRetries = options.PoolSize + 1 // https://github.com/go-redis/redis/issues/1737

	return NewClient(redis.NewClient(options), logger, &c.Options), nil
}

// GetAddr returns the Redis host:port or Unix socket address.
func (c *Client) GetAddr() string {
	return c.Client.Options().Addr
}

// HPair defines Redis hashes field-value pairs.
type HPair struct {
	Field string
	Value string
}

// HYield yields HPair field-value pairs for all fields in the hash stored at key.
func (c *Client) HYield(ctx context.Context, key string) (<-chan HPair, <-chan error) {
	pairs := make(chan HPair, c.Options.HScanCount)

	return pairs, com.WaitAsync(com.WaiterFunc(func() error {
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

	return pairs, com.WaitAsync(com.WaiterFunc(func() error {
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

type ctxDialerFunc = func(ctx context.Context, network, addr string) (net.Conn, error)

// dialWithLogging returns a Redis Dialer with logging capabilities.
func dialWithLogging(dialer ctxDialerFunc, logger *logging.Logger) ctxDialerFunc {
	// dial behaves like net.Dialer#DialContext,
	// but re-tries on common errors that are considered retryable.
	return func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
		err = retry.WithBackoff(
			ctx,
			func(ctx context.Context) (err error) {
				conn, err = dialer(ctx, network, addr)
				return
			},
			retry.Retryable,
			backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second),
			retry.Settings{
				Timeout: retry.DefaultTimeout,
				OnRetryableError: func(_ time.Duration, _ uint64, err, lastErr error) {
					if lastErr == nil || err.Error() != lastErr.Error() {
						logger.Warnw("Can't connect to Redis. Retrying", zap.Error(err))
					}
				},
				OnSuccess: func(elapsed time.Duration, attempt uint64, _ error) {
					if attempt > 1 {
						logger.Infow("Reconnected to Redis",
							zap.Duration("after", elapsed), zap.Uint64("attempts", attempt))
					}
				},
			},
		)

		err = errors.Wrap(err, "can't connect to Redis")

		return
	}
}
