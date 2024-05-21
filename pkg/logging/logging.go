package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"sync"
	"time"
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

// Logging implements access to a default logger and named child loggers.
// Log levels can be configured per named child via Options which, if not configured,
// fall back on a default log level.
// Logs either to the console or to systemd-journald.
type Logging struct {
	logger    *Logger
	output    string
	verbosity zap.AtomicLevel
	interval  time.Duration

	// coreFactory creates zapcore.Core based on the log level and the log output.
	coreFactory func(zap.AtomicLevel) zapcore.Core

	mu      sync.Mutex
	loggers map[string]*Logger

	options Options
}

// NewLogging takes the name and log level for the default logger,
// output where log messages are written to,
// options having log levels for named child loggers
// and returns a new Logging.
func NewLogging(name string, level zapcore.Level, output string, options Options, interval time.Duration) (*Logging, error) {
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

	logger := NewLogger(zap.New(coreFactory(verbosity)).Named(name).Sugar(), interval)

	return &Logging{
			logger:      logger,
			output:      output,
			verbosity:   verbosity,
			interval:    interval,
			coreFactory: coreFactory,
			loggers:     make(map[string]*Logger),
			options:     options,
		},
		nil
}

// NewLoggingFromConfig returns a new Logging from Config.
func NewLoggingFromConfig(name string, c Config) (*Logging, error) {
	return NewLogging(name, c.Level, c.Output, c.Options, c.Interval)
}

// GetChildLogger returns a named child logger.
// Log levels for named child loggers are obtained from the logging options and, if not found,
// set to the default log level.
func (l *Logging) GetChildLogger(name string) *Logger {
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

	logger := NewLogger(zap.New(l.coreFactory(verbosity)).Named(name).Sugar(), l.interval)
	l.loggers[name] = logger

	return logger
}

// GetLogger returns the default logger.
func (l *Logging) GetLogger() *Logger {
	return l.logger
}
