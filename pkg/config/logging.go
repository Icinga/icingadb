package config

import (
	"github.com/creasty/defaults"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/internal/logging"
)

// Logging defines Logger configuration.
type Logging struct {
	Level  string `yaml:"level" default:"debug"`
	Output string `yaml:"output" default:"console"`

	logging.Options `yaml:"options"`
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
