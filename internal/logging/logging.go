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

type Logging struct {
	enabled *atomic.Bool
	level   zap.AtomicLevel
	logger  *zap.SugaredLogger
	mu      sync.Mutex
	loggers map[string]*zap.SugaredLogger
	options Options
}

type Options map[string]string

func NewLogging(level string, options Options) *Logging {
	var lvl zapcore.Level
	if err := lvl.Set(level); err != nil {
		panic(err)
	}
	atom := zap.NewAtomicLevelAt(lvl)
	enabled := atomic.NewBool(true)

	logger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.Lock(os.Stderr),
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

func (l *Logging) GetChildLogger(name string) *zap.SugaredLogger {
	l.mu.Lock()
	defer l.mu.Unlock()

	if logger, ok := l.loggers[name]; ok {
		return logger
	}
	if level, found := l.options[name]; found {
		logger := l.logger.Desugar().With(zap.Skip()).WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
			childLvl := zapcore.LevelEnabler(levelMap[level])
			io := zapcore.AddSync(os.Stderr)
			return zapcore.NewCore(zapcore.NewConsoleEncoder(zap.NewDevelopmentConfig().EncoderConfig), io, childLvl)
		})).Sugar().Named(name)
		l.loggers[name] = logger
		return logger
	} else {
		logger := l.logger.Named(name)
		l.loggers[name] = logger
		return logger
	}
}

func (l *Logging) GetLogger() *zap.SugaredLogger {
	return l.logger
}

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
	os.Exit(1)
}

var levelMap = func() map[string]zapcore.Level {
	logLvl := make(map[string]zapcore.Level)

	for level, Value := range map[string]zapcore.Level{
		"debug":  zap.DebugLevel,
		"info":   zap.InfoLevel,
		"warn":   zap.WarnLevel,
		"error":  zap.ErrorLevel,
		"fatal":  zap.FatalLevel,
		"panic":  zap.PanicLevel,
		"dpanic": zap.DPanicLevel,
	} {
		logLvl[level] = Value
	}
	return logLvl
}()
