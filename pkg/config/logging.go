package config

import (
	"fmt"
	"github.com/creasty/defaults"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/internal/logging"
	"github.com/pkg/errors"
)

// Logging defines Logger configuration.
type Logging struct {
	Level string `yaml:"level" default:"debug"`

	logging.Options `yaml:"options"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (l *Logging) UnmarshalYAML(unmarshal func(interface{}) error) error {
	fmt.Println("i was here")
	if err := defaults.Set(l); err != nil {
		return errors.Wrap(err, "can't set default logging config")
	}
	// Prevent recursion.
	type self Logging
	if err := unmarshal((*self)(l)); err != nil {
		return internal.CantUnmarshalYAML(err, l)
	}
	return nil
}
