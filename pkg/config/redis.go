package config

import (
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"go.uber.org/zap"
	"time"
)

// Redis defines Redis client configuration.
type Redis struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
}

// NewClient prepares Redis client configuration,
// calls redis.NewClient, but returns *icingaredis.Client.
func (r *Redis) NewClient(logger *zap.SugaredLogger) (*icingaredis.Client, error) {
	c := redis.NewClient(&redis.Options{
		Addr:        r.Address,
		Password:    r.Password,
		DB:          0, // Use default DB,
		ReadTimeout: 30 * time.Second,
	})

	return icingaredis.NewClient(c, logger), nil
}
