package config

import (
	"github.com/icinga/icingadb/pkg/icingadb/history"
	"github.com/pkg/errors"
	"time"
)

// Retention defines configuration for history retention.
type Retention struct {
	HistoryDays uint64                   `yaml:"history-days" env:"HISTORY_DAYS"`
	SlaDays     uint64                   `yaml:"sla-days" env:"SLA_DAYS"`
	Interval    time.Duration            `yaml:"interval" env:"INTERVAL" default:"1h"`
	Count       uint64                   `yaml:"count" env:"COUNT" default:"5000"`
	Options     history.RetentionOptions `yaml:"options" env:"OPTIONS"`
}

// Validate checks constraints in the supplied retention configuration and
// returns an error if they are violated.
func (r *Retention) Validate() error {
	if r.Interval <= 0 {
		return errors.New("retention interval must be positive")
	}

	if r.Count == 0 {
		return errors.New("count must be greater than zero")
	}

	return r.Options.Validate()
}
