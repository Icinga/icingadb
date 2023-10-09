package redis

import (
	"github.com/icinga/icingadb/pkg/config"
	"github.com/pkg/errors"
)

// Config defines Config client configuration.
type Config struct {
	Host       string     `yaml:"host"`
	Port       int        `yaml:"port"`
	Database   int        `yaml:"database" default:"0"`
	Password   string     `yaml:"password"`
	TlsOptions config.TLS `yaml:",inline"`
	Options    Options    `yaml:"options"`
}

// Validate checks constraints in the supplied Config configuration and returns an error if they are violated.
func (r *Config) Validate() error {
	if r.Host == "" {
		return errors.New("Config host missing")
	}

	return r.Options.Validate()
}
