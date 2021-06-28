package logging

import (
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var levelMap = func() map[string]zapcore.Level {
	logLvl := make(map[string]zapcore.Level)

	for level, Value := range map[string]zapcore.Level{
		"debug":  zapcore.DebugLevel,
		"info":   zapcore.InfoLevel,
		"warn":   zapcore.WarnLevel,
		"error":  zapcore.ErrorLevel,
		"fatal":  zapcore.FatalLevel,
		"panic":  zapcore.PanicLevel,
		"dpanic": zapcore.DPanicLevel,
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

func NewLogger(level string, opts Options) *Logging {
	return &Logging{
		Level:   level,
		Options: opts,
		Map:     make(logger),
	}
}

func (l Logging) GetLogger(component string) *zap.SugaredLogger {
	if logger, found := l.Map[component]; found {
		return logger
	}

	loggerCfg := zap.NewDevelopmentConfig()

	loggerCfg.DisableStacktrace = true

	if level, found := l.Options[component]; found {
		loggerCfg.Level = zap.NewAtomicLevelAt(levelMap[level])
	} else {
		loggerCfg.Level = zap.NewAtomicLevelAt(levelMap[l.Level])
	}

	logger, err := loggerCfg.Build()
	if err != nil {
		utils.Fatal(errors.Wrap(err, "can't create logger"))
	}
	l.Map[component] = logger.Sugar()

	return l.Map[component]
}
