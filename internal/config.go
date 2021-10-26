package internal

import "time"

// LoggingInterval returns the interval for periodic logging.
func LoggingInterval() time.Duration {
	return c.LoggingInterval
}

// SetLoggingInterval configures the interval for periodic logging.
func SetLoggingInterval(i time.Duration) {
	c.LoggingInterval = i
}

var c config

type config struct {
	LoggingInterval time.Duration
}
