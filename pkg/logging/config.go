package logging

import (
	"fmt"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	"os"
	"time"
)

// Options define child loggers with their desired log level.
type Options map[string]zapcore.Level

// Config defines Logger configuration.
type Config struct {
	// zapcore.Level at 0 is for info level.
	Level  zapcore.Level `yaml:"level" default:"0"`
	Output string        `yaml:"output"`
	// Interval for periodic logging.
	Interval time.Duration `yaml:"interval" default:"20s"`

	Options `yaml:"options"`
}

// Validate checks constraints in the configuration and returns an error if they are violated.
// Also configures the log output if it is not configured:
// systemd-journald is used when Icinga DB is running under systemd, otherwise stderr.
func (l *Config) Validate() error {
	if l.Interval <= 0 {
		return errors.New("periodic logging interval must be positive")
	}

	if l.Output == "" {
		if _, ok := os.LookupEnv("NOTIFY_SOCKET"); ok {
			// When started by systemd, NOTIFY_SOCKET is set by systemd for Type=notify supervised services,
			// which is the default setting for the Icinga DB service.
			// This assumes that Icinga DB is running under systemd, so set output to systemd-journald.
			l.Output = JOURNAL
		} else {
			// Otherwise set it to console, i.e. write log messages to stderr.
			l.Output = CONSOLE
		}
	}

	// To be on the safe side, always call AssertOutput.
	return AssertOutput(l.Output)
}

// AssertOutput returns an error if output is not a valid logger output.
func AssertOutput(o string) error {
	if o == CONSOLE || o == JOURNAL {
		return nil
	}

	return invalidOutput(o)
}

func invalidOutput(o string) error {
	return fmt.Errorf("%s is not a valid logger output. Must be either %q or %q", o, CONSOLE, JOURNAL)
}
