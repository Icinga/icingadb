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
// Log levels can be configured per child via Options.
// Logs either to the console or the systemd journal.
// Provides Fatal log to stop all loggers before the error is logged and the program exits.
type Logging struct {
	// enabled implements a switch to disable logging on Fatal log during systemd-journal logging.
	// This switch is added to overcome the race condition during systemd-journal logging.
	// See: https://github.com/systemd/systemd/issues/2913.
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

// defaultEncConfig stores default zapcore.EncoderConfig for logging package.
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

// NewLogging returns a new Logging.
func NewLogging(level string, output string, options Options) *Logging {
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
		panic(fmt.Sprintf("%s is not a valid logger output", output))
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
	}
}

// GetChildLogger returns a named child logger.
// Log levels for the named child loggers are obtained from Logging.options. if the level for the corresponding
// component is not found in Logging.options then we set the named child logger to default logger and return it.
// If the child logger is found in Logging.loggers then that corresponding named child logger is returned.
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

// Fatal logs fatal IcingaDb messages.
// In case of fatal errors panic() does not seem to function properly, neither when the log messages are written to
// console nor when they are sent to systemd-journal (i.e, the some of the error message is lost). Similarly,
// the process exits without showing the entire error message while using zap.logger.Fatal().
// Hence, a custom Fatal method has been written by introducing a short buffer time between Fatal log and os.Exit(1)
// by sleeping for a short interval after logging fatal error in case of systemd-journal logging and then the process
// is exited. Hence avoiding the loss of fatal level log messages on exit on failure.
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

	// A short buffer time between Fatal log and os.Exit(1) by sleeping in case of systemd-journal logging.
	// This is done to overcome the race condition during systemd-journal logging, which causes systemd journal unable
	// to attribute messages incoming from processes that exited to their cgroup.
	// See: https://github.com/systemd/systemd/issues/2913.
	if l.output == "systemd-journal" {
		time.Sleep(time.Millisecond * 1100)
	}

	os.Exit(ExitFailure)
}
