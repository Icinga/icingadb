package config

import (
	"context"
	"github.com/creasty/defaults"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/backoff"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/retry"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"net"
	"os"
	"sync"
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

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (r *Redis) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := defaults.Set(r); err != nil {
		return errors.Wrapf(err, "can't set defaults %#v", r)
	}
	// Prevent recursion.
	type self Redis
	if err := unmarshal((*self)(r)); err != nil {
		return internal.CantUnmarshalYAML(err, r)
	}

	return nil
}

// dialWithLogging returns a Redis Dialer with logging capabilities.
func dialWithLogging(logger *zap.SugaredLogger) func(context.Context, string, string) (net.Conn, error) {
	// dial behaves like net.Dialer#DialContext, but re-tries on syscall.ECONNREFUSED.
	return func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
		var dl net.Dialer
		var logFirstError sync.Once

		err = retry.WithBackoff(
			ctx,
			func(ctx context.Context) (err error) {
				conn, err = dl.DialContext(ctx, network, addr)
				logFirstError.Do(func() {
					if err != nil {
						logger.Warnw("Can't connect to Redis. Retrying", zap.Error(err))
					}
				})
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
			5*time.Minute,
		)

		err = errors.Wrap(err, "can't connect to Redis")

		return
	}
}
