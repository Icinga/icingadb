package config

import (
	"github.com/icinga/icingadb/pkg/logging"
	"go.uber.org/zap/zapcore"
	"os"
)

// Logging defines Logger configuration.
type Logging struct {
	// zapcore.Level at 0 is for info level.
	Level  zapcore.Level `yaml:"level" default:"0"`
	Output string        `yaml:"output"`

	logging.Options `yaml:"options"`
}

// Validate checks constraints in the supplied Logging configuration and returns an error if they are violated.
// Also configures the log output if it is not configured:
// systemd-journald is used when Icinga DB is running under systemd, otherwise stderr.
func (l *Logging) Validate() error {
	if l.Output == "" {
		if _, ok := os.LookupEnv("NOTIFY_SOCKET"); ok {
			// When started by systemd, NOTIFY_SOCKET is set by systemd for Type=notify supervised services,
			// which is the default setting for the Icinga DB service.
			// This assumes that Icinga DB is running under systemd, so set output to systemd-journald.
			l.Output = logging.JOURNAL
		} else {
			// Otherwise set it to console, i.e. write log messages to stderr.
			l.Output = logging.CONSOLE
		}
	}

	// To be on the safe side, always call AssertOutput.
	return logging.AssertOutput(l.Output)
}
