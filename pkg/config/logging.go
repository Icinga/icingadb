package config

import (
	"github.com/creasty/defaults"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/internal/logging"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
)

// Logging defines Logger configuration.
type Logging struct {
	// zapcore.Level at 0 is for info level. So zapcore.Level at -1 represents debug
	// Here Level is set to debug by default.
	Level zapcore.Level `yaml:"level" default:"-1"`

	logging.Options `yaml:"options"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (l *Logging) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
