package redis

import (
	"github.com/icinga/icingadb/pkg/config"
	"github.com/pkg/errors"
	"time"
)

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

// Config defines Config client configuration.
type Config struct {
	Host       string     `yaml:"host"`
	Port       int        `yaml:"port"`
	Password   string     `yaml:"password"`
	TlsOptions config.TLS `yaml:",inline"`
	Options    Options    `yaml:"options"`
}

// Validate checks constraints in the supplied Config configuration and returns an error if they are violated.
func (r *Config) Validate() error {
	if r.Host == "" {
		return errors.New("Redis host missing")
	}

	return r.Options.Validate()
}
