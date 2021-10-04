package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"sync"
)

// Logging implements access to a default logger and named child loggers.
// Log levels can be configured per named child via Options which, if not configured,
// fall back on a default log level.
type Logging struct {
	level  zap.AtomicLevel
	logger *zap.SugaredLogger
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
type Options map[string]zapcore.Level

// NewLogging takes the name and log level for the default logger,
// options having log levels for named child loggers
// and returns a new Logging.
func NewLogging(name string, level zapcore.Level, options Options) (*Logging, error) {
	atom := zap.NewAtomicLevelAt(level)

	encoder := zapcore.NewConsoleEncoder(defaultEncConfig)
	syncer := zapcore.Lock(os.Stderr)

	logger := zap.New(zapcore.NewCore(
		encoder,
		syncer,
		atom,
	)).Named(name)

	return &Logging{
			level:   atom,
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
		atom := zap.NewAtomicLevelAt(level)

		logger := l.logger.Desugar().WithOptions(
			zap.WrapCore(func(c zapcore.Core) zapcore.Core {
				return zapcore.NewCore(
					l.encoder,
					l.syncer,
					atom,
				)
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
