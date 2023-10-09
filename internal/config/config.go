package config

import (
	"github.com/icinga/icingadb/pkg/database"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/redis"
	"github.com/pkg/errors"
	"time"
)

// Config defines Icinga DB config.
type Config struct {
	Database  database.Config `yaml:"database"`
	Redis     redis.Config    `yaml:"redis"`
	Logging   logging.Config  `yaml:"logging"`
	Retention RetentionConfig `yaml:"retention"`
}

// Validate checks constraints in the supplied configuration and returns an error if they are violated.
func (c *Config) Validate() error {
	if err := c.Database.Validate(); err != nil {
		return err
	}
	if err := c.Redis.Validate(); err != nil {
		return err
	}
	if err := c.Logging.Validate(); err != nil {
		return err
	}
	if err := c.Retention.Validate(); err != nil {
		return err
	}

	return nil
}

// Flags defines CLI flags.
type Flags struct {
	// Version decides whether to just print the version and exit.
	Version bool `long:"version" description:"print version and exit"`
	// Config is the path to the config file
	Config string `short:"c" long:"config" description:"path to config file" required:"true" default:"/etc/icingadb/config.yml"`
}

// RetentionConfig defines configuration for history retention.
type RetentionConfig struct {
	HistoryDays uint64                   `yaml:"history-days"`
	SlaDays     uint64                   `yaml:"sla-days"`
	Interval    time.Duration            `yaml:"interval" default:"1h"`
	Count       uint64                   `yaml:"count" default:"5000"`
	Options     history.RetentionOptions `yaml:"options"`
}

// Validate checks constraints in the supplied retention configuration and
// returns an error if they are violated.
func (r *RetentionConfig) Validate() error {
	if r.Interval <= 0 {
		return errors.New("retention interval must be positive")
	}

	if r.Count == 0 {
		return errors.New("count must be greater than zero")
	}

	return r.Options.Validate()
}
