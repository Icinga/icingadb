package config

import (
	"context"
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
	Address  string               `yaml:"address"`
	Password string               `yaml:"password"`
	Options  *icingaredis.Options `yaml:"options"`
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

	return icingaredis.NewClient(c, logger, r.Options), nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (r *Redis) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Prevent recursion.
	type T Redis
	self := (*T)(r)

	if err := unmarshal(&self); err != nil {
		return internal.CantUnmarshalYAML(err, r)
	}
	if self.Options == nil {
		// Options is nil if no option is set in the configuration.
		// in order for the default values to be set.
		// We have to call unmarshal ourselves to trigger the Options.UnmarshalYAML call
		if err := unmarshal(&self.Options); err != nil {
			return internal.CantUnmarshalYAML(err, r)
		}
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
