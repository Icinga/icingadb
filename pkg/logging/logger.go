package logging

import (
	"go.uber.org/zap"
	"time"
)

// Logger wraps zap.SugaredLogger and
// allows to get the interval for periodic logging.
type Logger struct {
	*zap.SugaredLogger
	interval time.Duration
}

// NewLogger returns a new Logger.
func NewLogger(base *zap.SugaredLogger, interval time.Duration) *Logger {
	return &Logger{
		SugaredLogger: base,
		interval:      interval,
	}
}

// Interval returns the interval for periodic logging.
func (l *Logger) Interval() time.Duration {
	return l.interval
}
