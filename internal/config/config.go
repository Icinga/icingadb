package config

import (
	"github.com/creasty/defaults"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icingadb/pkg/icingadb/history"
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

func (c *Config) SetDefaults() {
	// Since SetDefaults() is called after the default values of the struct's fields have been evaluated,
	// setting the default port only works here because
	// the embedded Redis config struct itself does not provide a default value.
	if defaults.CanUpdate(c.Redis.Port) {
		c.Redis.Port = 6380
	}
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
	HistoryDays uint16                   `yaml:"history-days"`
	SlaDays     uint16                   `yaml:"sla-days"`
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
