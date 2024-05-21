package config

import (
	"github.com/icinga/icingadb/pkg/database"
	"github.com/icinga/icingadb/pkg/logging"
)

// Config defines Icinga DB config.
type Config struct {
	Database  database.Config `yaml:"database"`
	Redis     Redis           `yaml:"redis"`
	Logging   logging.Config  `yaml:"logging"`
	Retention Retention       `yaml:"retention"`
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
