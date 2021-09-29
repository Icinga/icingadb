package logging

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"sync"
)

const (
	CONSOLE = "console"
	JOURNAL = "systemd-journald"
)

// defaultEncConfig defines the default zapcore.EncoderConfig for the logging package.
var defaultEncConfig = zapcore.EncoderConfig{
	TimeKey:        "ts",
	LevelKey:       "level",
	NameKey:        "logger",
	CallerKey:      "caller",
	MessageKey:     "msg",
	StacktraceKey:  "stacktrace",
	LineEnding:     zapcore.DefaultLineEnding,
	EncodeLevel:    zapcore.CapitalLevelEncoder,
	EncodeTime:     zapcore.ISO8601TimeEncoder,
	EncodeDuration: zapcore.StringDurationEncoder,
	EncodeCaller:   zapcore.ShortCallerEncoder,
}

// Options define child loggers with their desired log level.
type Options map[string]zapcore.Level

// Logging implements access to a default logger and named child loggers.
// Log levels can be configured per named child via Options which, if not configured,
// fall back on a default log level.
// Logs either to the console or to systemd-journald.
type Logging struct {
	logger    *zap.SugaredLogger
	output    string
	verbosity zap.AtomicLevel

	// coreFactory creates zapcore.Core based on the log level and the log output.
	coreFactory func(zap.AtomicLevel) zapcore.Core

	mu      sync.Mutex
	loggers map[string]*zap.SugaredLogger

	options Options
}

// NewLogging takes the name and log level for the default logger,
// output where log messages are written to,
// options having log levels for named child loggers
// and returns a new Logging.
func NewLogging(name string, level zapcore.Level, output string, options Options) (*Logging, error) {
	verbosity := zap.NewAtomicLevelAt(level)

	var coreFactory func(zap.AtomicLevel) zapcore.Core
	switch output {
	case CONSOLE:
		enc := zapcore.NewConsoleEncoder(defaultEncConfig)
		ws := zapcore.Lock(os.Stderr)
		coreFactory = func(verbosity zap.AtomicLevel) zapcore.Core {
			return zapcore.NewCore(enc, ws, verbosity)
		}
	case JOURNAL:
		coreFactory = func(verbosity zap.AtomicLevel) zapcore.Core {
			return NewJournaldCore(name, verbosity)
		}
	default:
		return nil, invalidOutput(output)
	}

	logger := zap.New(coreFactory(verbosity)).Named(name).Sugar()

	return &Logging{
			logger:      logger,
			output:      output,
			verbosity:   verbosity,
			coreFactory: coreFactory,
			loggers:     map[string]*zap.SugaredLogger{},
			options:     options,
		},
		nil
}

// GetChildLogger returns a named child logger.
// Log levels for named child loggers are obtained from the logging options and, if not found,
// set to the default log level.
func (l *Logging) GetChildLogger(name string) *zap.SugaredLogger {
	l.mu.Lock()
	defer l.mu.Unlock()

	if logger, ok := l.loggers[name]; ok {
		return logger
	}

	var verbosity zap.AtomicLevel
	if level, found := l.options[name]; found {
		verbosity = zap.NewAtomicLevelAt(level)
	} else {
		verbosity = l.verbosity
	}

	logger := zap.New(l.coreFactory(verbosity)).Named(name).Sugar()
	l.loggers[name] = logger

	return logger
}

// GetLogger returns the default logger.
func (l *Logging) GetLogger() *zap.SugaredLogger {
	return l.logger
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
