package config

import (
	"github.com/creasty/defaults"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/internal/logging"
)

// Logging defines Logger configuration.
type Logging struct {
	Level           string `yaml:"level" default:"debug"`
	logging.Options `yaml:"options,omitempty"`
}

// NewLogging prepares Logging configuration,
// calls logging.NewLogging, and returns *logging.Logging.
func (l *Logging) NewLogging() *logging.Logging {
	level := l.Level
	return logging.NewLogging(level, l.Options)
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (l *Logging) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := defaults.Set(l); err != nil {
		return err
	}
	// Prevent recursion.
	type self Logging
	if err := unmarshal((*self)(l)); err != nil {
		return internal.CantUnmarshalYAML(err, l)
	}
	return nil
}
