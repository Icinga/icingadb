package config

import (
	"errors"
	"github.com/creasty/defaults"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"go.uber.org/zap"
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
		Password:    r.Password,
		DB:          0, // Use default DB,
		ReadTimeout: r.Timeout,
	})

	return icingaredis.NewClient(c, logger, &r.Options), nil
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
