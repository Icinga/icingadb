package config

import (
	"github.com/creasty/defaults"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/internal/logging"
)

// Logging defines Logger configuration.
type Logging struct {
	Level           string `yaml:"level" default:"info"`
	logging.Options `yaml:"options,omitempty"`
}

// NewLogger prepares Cleanup configuration,
// calls logging.NewLogger, but returns *logging.Logging.
func (l *Logging) NewLogger() (*logging.Logging, error) {
	level := l.Level
	return logging.NewLogging(level, l.Options), nil
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
