package logging

import (
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	DefaultLogLevel = "info"
)

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

type Logging struct {
	Level string

	Options Options
	Map     logger
}

type logger map[string]*zap.SugaredLogger

type Options map[string]string

func NewLogging(level string, opts Options) *Logging {
	return &Logging{
		Level:   level,
		Options: opts,
		Map:     make(logger),
	}
}

func (l Logging) GetLogger(component string) *zap.SugaredLogger {
	var logger *zap.Logger
	if logger, found := l.Map[component]; found {
		return logger
	}

	if level, found := l.Options[component]; found {
		logger = NewLogger(level)
	} else {
		logger = NewLogger(l.Level)
	}

	l.Map[component] = logger.Sugar()

	return l.Map[component]
}

func NewLogger(lvl ...string) *zap.Logger {
	var level zap.AtomicLevel
	if len(lvl) > 0 {
		level = zap.NewAtomicLevelAt(levelMap[lvl[0]])
	} else {
		level = zap.NewAtomicLevelAt(levelMap[DefaultLogLevel])
	}

	loggerCfg := zap.NewDevelopmentConfig()

	loggerCfg.DisableStacktrace = true

	loggerCfg.Level = level

	logger, err := loggerCfg.Build()
	if err != nil {
		utils.Fatal(errors.Wrap(err, "can't create logger"))
	}

	return logger
}
