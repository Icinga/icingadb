package config

import (
	"context"
	"errors"
	"github.com/creasty/defaults"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/pkg/backoff"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/retry"
	"go.uber.org/zap"
	"net"
	"os"
	"syscall"
	"time"
)

// Redis defines Redis client configuration.
type Redis struct {
	Address             string `yaml:"address"`
	Password            string `yaml:"password"`
	icingaredis.Options `yaml:",inline"`
}

// NewClient prepares Redis client configuration,
// calls redis.NewClient, but returns *icingaredis.Client.
func (r *Redis) NewClient(logger *zap.SugaredLogger) (*icingaredis.Client, error) {
	c := redis.NewClient(&redis.Options{
		Addr:        r.Address,
		Dialer:      dial,
		Password:    r.Password,
		DB:          0, // Use default DB,
		ReadTimeout: r.Timeout,
	})

	opts := c.Options()
	opts.MaxRetries = opts.PoolSize + 1 // https://github.com/go-redis/redis/issues/1737
	c = redis.NewClient(opts)

	return icingaredis.NewClient(c, logger, &r.Options), nil
}

// dial behaves like net.Dialer#DialContext, but re-tries on syscall.ECONNREFUSED.
func dial(ctx context.Context, network, addr string) (conn net.Conn, err error) {
	var dl net.Dialer

	timeoutCtx, cancelTimeoutCtx := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelTimeoutCtx()

	_ = retry.WithBackoff(
		timeoutCtx,
		func() error {
			prevErr := err
			conn, err = dl.DialContext(timeoutCtx, network, addr)

			if prevErr != nil && errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				err = prevErr
			}

			return err
		},
		func(err error) bool {
			if op, ok := err.(*net.OpError); ok {
				sys, ok := op.Err.(*os.SyscallError)
				return ok && sys.Err == syscall.ECONNREFUSED
			}
			return false
		},
		backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second),
	)
	return
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (d *Redis) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := defaults.Set(d); err != nil {
		return err
	}
	// Prevent recursion.
	type self Redis
	if err := unmarshal((*self)(d)); err != nil {
		return err
	}

	if d.MaxHMGetConnections < 1 {
		return errors.New("max_hmget_connections must be at least 1")
	}
	if d.HMGetCount < 1 {
		return errors.New("hmget_count must be at least 1")
	}
	if d.HScanCount < 1 {
		return errors.New("hscan_count must be at least 1")
	}

	return nil
}
