package config

import (
	"github.com/icinga/icingadb/pkg/logging"
	"go.uber.org/zap/zapcore"
)

// Logging defines Logger configuration.
type Logging struct {
	// zapcore.Level at 0 is for info level.
	Level zapcore.Level `yaml:"level" default:"0"`

	logging.Options `yaml:"options"`
}
