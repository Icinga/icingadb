package config

import (
	"github.com/creasty/defaults"
	"github.com/goccy/go-yaml"
	"github.com/icinga/icingadb/pkg/database"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"os"
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

// FromYAMLFile returns a new Config value created from the given YAML config file.
func FromYAMLFile(name string) (*Config, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, errors.Wrap(err, "can't open YAML file "+name)
	}
	defer f.Close()

	c := &Config{}
	d := yaml.NewDecoder(f, yaml.DisallowUnknownField())

	if err := defaults.Set(c); err != nil {
		return nil, errors.Wrap(err, "can't set config defaults")
	}

	if err := d.Decode(c); err != nil {
		return nil, errors.Wrap(err, "can't parse YAML file "+name)
	}

	if err := c.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}

	return c, nil
}

// ParseFlags parses CLI flags and
// returns a Flags value created from them.
func ParseFlags() (*Flags, error) {
	f := &Flags{}
	parser := flags.NewParser(f, flags.Default)

	if _, err := parser.Parse(); err != nil {
		return nil, errors.Wrap(err, "can't parse CLI flags")
	}

	return f, nil
}
