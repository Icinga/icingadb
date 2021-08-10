package config

import (
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	"os"
)

// Config defines Icinga DB config.
type Config struct {
	Database Database `yaml:"database"`
	Redis    Redis    `yaml:"redis"`
	Cleanup  Cleanup  `yaml:"cleanup"`
}

// Flags defines CLI flags.
type Flags struct {
	// Config is the path to the config file
	Config string `short:"c" long:"config" description:"path to config file" required:"true" default:"./config.yml"`
}

// FromYAMLFile returns a new Config value created from the given YAML config file.
func FromYAMLFile(name string) (*Config, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, errors.Wrap(err, "can't open YAML file "+name)
	}
	defer f.Close()

	c := &Config{}
	d := yaml.NewDecoder(f)

	if err := d.Decode(c); err != nil {
		return nil, errors.Wrap(err, "can't parse YAML file "+name)
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
