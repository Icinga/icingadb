package config

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/pkg/backoff"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/retry"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"net"
	"os"
	"syscall"
	"time"
)

// Redis defines Redis client configuration.
type Redis struct {
	Address  string              `yaml:"address"`
	Password string              `yaml:"password"`
	Options  icingaredis.Options `yaml:"options"`
}

// NewClient prepares Redis client configuration,
// calls redis.NewClient, but returns *icingaredis.Client.
func (r *Redis) NewClient(logger *zap.SugaredLogger) (*icingaredis.Client, error) {
	c := redis.NewClient(&redis.Options{
		Addr:        r.Address,
		Dialer:      dialWithLogging(logger),
		Password:    r.Password,
		DB:          0, // Use default DB,
		ReadTimeout: r.Options.Timeout,
	})

	opts := c.Options()
	opts.MaxRetries = opts.PoolSize + 1 // https://github.com/go-redis/redis/issues/1737
	c = redis.NewClient(opts)

	return icingaredis.NewClient(c, logger, &r.Options), nil
}

// dialWithLogging returns a Redis Dialer with logging capabilities.
func dialWithLogging(logger *zap.SugaredLogger) func(context.Context, string, string) (net.Conn, error) {
	// dial behaves like net.Dialer#DialContext, but re-tries on syscall.ECONNREFUSED.
	return func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
		var dl net.Dialer

		err = retry.WithBackoff(
			ctx,
			func(ctx context.Context) (err error) {
				conn, err = dl.DialContext(ctx, network, addr)
				return
			},
			func(err error) bool {
				if op, ok := err.(*net.OpError); ok {
					sys, ok := op.Err.(*os.SyscallError)
					return ok && sys.Err == syscall.ECONNREFUSED
				}
				return false
			},
			backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second),
			retry.Settings{
				Timeout: 5 * time.Minute,
				OnError: func(_ time.Duration, _ uint64, err, lastErr error) {
					if lastErr == nil || err.Error() != lastErr.Error() {
						logger.Warnw("Can't connect to Redis. Retrying", zap.Error(err))
					}
				},
			},
		)

		err = errors.Wrap(err, "can't connect to Redis")

		return
	}
}

// Validate checks constraints in the supplied Redis configuration and returns an error if they are violated.
func (r *Redis) Validate() error {
	return r.Options.Validate()
}
