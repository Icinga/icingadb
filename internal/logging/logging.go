package logging

import (
	"fmt"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"sync"
	"time"
)

// Logging implements access to a default logger and named child loggers.
// Log levels can be configured per named child via Options which, if not configured,
// fall back on a default log level.
// Logs either to the console or to the systemd journal.
// Provides Fatal log for consistent handling of fatal errors that
// exit the program after being logged.
type Logging struct {
	// enabled implements a switch to disable logging during Fatal log if
	// the output is set to the systemd journal, because we then sleep after logging and
	// before exiting the program, which could otherwise add further log messages during this time.
	// See Fatal for details.
	enabled *atomic.Bool
	level   zap.AtomicLevel
	output  string
	logger  *zap.SugaredLogger
	// encoder defines the zapcore.Encoder,
	// which is used to create the default logger and the child loggers
	encoder zapcore.Encoder
	// syncer defines the zapcore.WriterSyncer,
	// which is used to create the default logger and the child loggers
	syncer  zapcore.WriteSyncer
	mu      sync.Mutex
	loggers map[string]*zap.SugaredLogger
	options Options
}

// defaultEncConfig stores default zapcore.EncoderConfig for this package.
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
type Options map[string]string

// NewLogging takes log level for default logger, output where log messages are written to
// and options having log levels for named child loggers and initializes a new Logging.
func NewLogging(level string, output string, options Options) (*Logging, error) {
	var lvl zapcore.Level
	if err := lvl.Set(level); err != nil {
		panic(err)
	}
	atom := zap.NewAtomicLevelAt(lvl)
	enabled := atomic.NewBool(true)

	var encoder zapcore.Encoder
	var syncer zapcore.WriteSyncer
	switch output {
	case "console":
		encoder = zapcore.NewConsoleEncoder(defaultEncConfig)
		syncer = zapcore.Lock(os.Stderr)
	case "systemd-journal":
		wr, err := newJournalWriter(os.Stderr, defaultEncConfig)
		if err != nil {
			panic(err)
		}
		encoder = zapcore.NewJSONEncoder(defaultEncConfig)
		syncer = zapcore.AddSync(wr)
	default:
		logger := zap.New(zapcore.NewCore(
			zapcore.NewConsoleEncoder(defaultEncConfig),
			zapcore.Lock(os.Stderr),
			zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				// Always log fatals, but allow other levels to be completely disabled.
				return zap.FatalLevel.Enabled(lvl) || enabled.Load() && atom.Enabled(lvl)
			}),
		))
		return &Logging{
				enabled: enabled,
				level:   atom,
				output:  output,
				logger:  logger.Sugar(),
				loggers: map[string]*zap.SugaredLogger{},
			},
			fmt.Errorf("%s is not a valid logger output", output)
	}

	logger := zap.New(zapcore.NewCore(
		encoder,
		syncer,
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			// Always log fatals, but allow other levels to be completely disabled.
			return zap.FatalLevel.Enabled(lvl) || enabled.Load() && atom.Enabled(lvl)
		}),
	))

	return &Logging{
			enabled: enabled,
			level:   atom,
			output:  output,
			logger:  logger.Sugar(),
			encoder: encoder,
			syncer:  syncer,
			loggers: map[string]*zap.SugaredLogger{},
			options: options,
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

	if level, found := l.options[name]; found {
		var lvl zapcore.Level
		if err := lvl.Set(level); err != nil {
			panic(err)
		}
		atom := zap.NewAtomicLevelAt(lvl)

		logger := l.logger.Desugar().WithOptions(
			zap.WrapCore(func(c zapcore.Core) zapcore.Core {
				return zapcore.NewCore(
					l.encoder,
					l.syncer,
					zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
						// Always log fatals, but allow other levels to be completely disabled.
						return zap.FatalLevel.Enabled(lvl) || l.enabled.Load() && atom.Enabled(lvl)
					}))
			})).Sugar().Named(name)

		l.loggers[name] = logger

		return logger
	}

	logger := l.logger.Named(name)
	l.loggers[name] = logger

	return logger
}

// GetLogger returns the default logger.
func (l *Logging) GetLogger() *zap.SugaredLogger {
	return l.logger
}

const (
	ExitFailure = 1
)

// Fatal logs fatal errors and exits afterwards.
//
// Sleeps for a short time before exiting if the output is set to systemd-journal,
// as systemd may not correctly attribute all or part of the log message due to a race condition
// and therefore it is missing when checking logs via journalctl -u icingadb:
// https://github.com/systemd/systemd/issues/2913
//
// Without sleep, we observed the following exit behavior when starting Icinga DB via systemd:
//
// • All or part of the log message is lost,
//  regardless of whether it is logged to stderr or to the journal.
//
// • When logging to stderr after also logging to the journal,
//   the message seems to be attributed correctly,
//   but this is actually not an option,
//   as we only want to log to the journal.
//
// • Sleep before exiting helps systemd to attribute the message correctly.
func (l *Logging) Fatal(err error) {
	logger := l.GetLogger().Desugar()

	if l.output == "systemd-journal" {
		l.enabled.Store(false)
		// Fatal would exit after write, but we want to sleep first in case of systemd-journal logging.
		logger = logger.WithOptions(zap.OnFatal(zapcore.WriteThenGoexit))
	}

	logged := make(chan struct{})
	go func() {
		defer close(logged)
		defer logger.Sync()
		logger.Fatal(fmt.Sprintf("%+v", err))
	}()
	<-logged

	// A short buffer time between Fatal log and os.Exit() by sleeping in case of systemd-journal logging.
	// This is done to overcome a race condition that results in systemd no longer attributing messages from processes
	// that have exited to their cgroup.
	// See: https://github.com/systemd/systemd/issues/2913.
	if l.output == "systemd-journal" {
		time.Sleep(time.Millisecond * 1100)
	}

	os.Exit(ExitFailure)
}
