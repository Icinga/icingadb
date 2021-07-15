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

// Logging implements logging using: a switch to disable logging on Fatal log, default logger and level,
// and stored child loggers and their options.
type Logging struct {
	// enabled implements a switch to disable logging on Fatal log.
	enabled *atomic.Bool
	// level defines default log-level.
	level zap.AtomicLevel
	// logger defines default logger.
	logger *zap.SugaredLogger

	mu sync.Mutex
	// loggers stores named child loggers.
	loggers map[string]*zap.SugaredLogger
	options Options
}

// Options stores a `child-logger-name: level` pair mapping.
type Options map[string]string

// consoleEncoder initializes a console encoder using zap.NewDevelopmentEncoderConfig().
var consoleEncoder = zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())

// syncer initializes WriteSyncer
var syncer = zapcore.Lock(os.Stderr)

// NewLogging returns a initialized Logging
// using level and options from configuration file.
func NewLogging(level string, options Options) *Logging {
	var lvl zapcore.Level
	if err := lvl.Set(level); err != nil {
		panic(err)
	}
	atom := zap.NewAtomicLevelAt(lvl)
	enabled := atomic.NewBool(true)

	logger := zap.New(zapcore.NewCore(
		consoleEncoder,
		syncer,
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			// Always log fatals, but allow other levels to be completely disabled.
			return zap.FatalLevel.Enabled(lvl) || enabled.Load() && atom.Enabled(lvl)
		}),
	))

	return &Logging{
		enabled: enabled,
		level:   atom,
		logger:  logger.Sugar(),
		mu:      sync.Mutex{},
		loggers: map[string]*zap.SugaredLogger{},
		options: options,
	}
}

// GetChildLogger returns a named child sugared logger for the default logger `Logging.logger`.
func (l *Logging) GetChildLogger(name string) *zap.SugaredLogger {
	l.mu.Lock()
	defer l.mu.Unlock()

	// if the logger is already stored then use the stored child logger.
	if logger, ok := l.loggers[name]; ok {
		return logger
	}

	if level, found := l.options[name]; found {
		var childlvl zapcore.Level
		if err := childlvl.Set(level); err != nil {
			panic(err)
		}
		atom := zap.NewAtomicLevelAt(childlvl)

		// child logger cloned from default logger and using childlvl for LevelEnablerFunc.
		logger := l.logger.Desugar().With(zap.Skip()).WithOptions(
			zap.WrapCore(func(c zapcore.Core) zapcore.Core {
				return zapcore.NewCore(
					consoleEncoder,
					syncer,
					zap.LevelEnablerFunc(func(childlvl zapcore.Level) bool {
						// Always log fatals, but allow other levels to be completely disabled.
						return zap.FatalLevel.Enabled(childlvl) || l.enabled.Load() && atom.Enabled(childlvl)
					}))
			})).Sugar().Named(name)

		l.loggers[name] = logger
		return logger
	} else {
		logger := l.logger.Named(name)
		l.loggers[name] = logger
		return logger
	}
}

// GetLogger returns the default sugared logger for the initialized Logging instance.
func (l *Logging) GetLogger() *zap.SugaredLogger {
	return l.logger
}

// Fatalf introduces a short buffer time between Fatal log and os.Exit(1) by sleeping
// for a short interval after logging fatal message. This is done to avoid the loss of fatal level log messages
// on exit on failure.
func (l *Logging) Fatalf(template string, args ...interface{}) {
	l.enabled.Store(false)

	// Fatal would exit after write, but we want to sleep first.
	logger := l.GetLogger().Desugar().WithOptions(zap.OnFatal(zapcore.WriteThenGoexit))

	logged := make(chan struct{})
	go func() {
		defer close(logged)
		defer logger.Sync()
		logger.Fatal(fmt.Sprintf(template, args...))
	}()
	<-logged

	time.Sleep(time.Millisecond * 1100)
	// exit on failure (ExitFailure).
	os.Exit(1)
}
